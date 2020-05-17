package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// OK logs unexpected errors and returns `true` for non-errors
func OK(err error) bool {
	if err != nil {
		log.Printf("unexpected error: %s", err)
		return false
	}

	return true
}

type Bot struct {
	API  *slack.Client
	User string

	// Events retrieves incoming Slack events; type switch these to e.g. *slackevents.MessageEvent to
	// read incoming messages on channels.
	Events chan slackevents.EventsAPIInnerEvent

	listener     *http.Server
	userCache    map[string]*slack.User
	channelCache map[string]*slack.Channel
	cacheLock    sync.Mutex
}

// New returns a *Bot, only if the provided xoxb- key is valid and retrieves a bot user; it should
// have at least channels.read, channels.history, chat.write, and users.read; `localURI` is the
// http://host:port/path that the event callback server will listen on.
//
// If New returns non-nil Bot, it's set up the Bot.Events channel and is attempting to listen on
// `localURI` in a goroutine, which will close the Bot.Events channel if listening for connections
// ever fails.
//
// Read Bot.Events for incoming events. Use Bot.API to make direct calls to Slack.
func New(xoxb, localURI string) (*Bot, error) {
	url, err := url.Parse(localURI)
	if err != nil {
		return nil, err
	}

	if url.Path == "" {
		return nil, fmt.Errorf("malformed URL")
	}

	ret := &Bot{
		API:    slack.New(xoxb),
		Events: make(chan slackevents.EventsAPIInnerEvent),
		listener: &http.Server{
			Addr: strings.Replace(url.Host, "localhost", "", 1),
		},
		userCache:    map[string]*slack.User{},
		channelCache: map[string]*slack.Channel{},
	}

	id, err := ret.API.AuthTest()
	if err != nil {
		return nil, err
	}
	ret.User = id.UserID

	go func() {
		http.HandleFunc(url.Path, ret.eventInbound)

		if err = ret.listener.ListenAndServe(); !OK(err) {
			close(ret.Events)
		}
	}()

	return ret, nil
}

// handle incoming Slack events
func (b *Bot) eventInbound(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if !OK(err) {
		return
	}

	outerEv, err := slackevents.ParseEvent(json.RawMessage(buf), slackevents.OptionNoVerifyToken())
	if !OK(err) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	if outerEv.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err = json.Unmarshal(buf, &r)
		if !OK(err) {
			w.WriteHeader(http.StatusInternalServerError)
		}

		json.NewEncoder(w).Encode(&struct {
			Challenge string `json:"challenge"`
		}{r.Challenge})

		return
	}

	if outerEv.Type != slackevents.CallbackEvent {
		log.Printf("unexpected event: %+v", outerEv)
		return
	}

	b.Events <- outerEv.InnerEvent
}

// Stop the server and wind down its associated goroutine
func (b *Bot) Stop() {
	b.listener.Shutdown(context.TODO())
}

// GetUser retrieves the User associated with a Slack ID.
//
// This is a convenience method (you can just use Bot.API to make API calls) that
// caches results; it returns no errors, but just nil Users when calls fail.
func (b *Bot) GetUser(id string) *slack.User {
	b.cacheLock.Lock()
	defer b.cacheLock.Unlock()

	if u, ok := b.userCache[id]; ok {
		return u
	}

	u, err := b.API.GetUserInfo(id)
	if !OK(err) {
		return nil
	}

	b.userCache[id] = u
	return u
}

// GetUserName maps Slack IDs to usernames.
func (b *Bot) GetUserName(id string) string {
	u := b.GetUser(id)
	if u == nil {
		return id
	}
	return u.Name
}

// GetChannel retrieves the Channel associated with a Slack ID.
//
// This is a convenience method (you can just use Bot.API to make API calls) that
// caches results; it returns no errors, but just nil Channels when calls fail.
func (b *Bot) GetChannel(id string) *slack.Channel {
	b.cacheLock.Lock()
	defer b.cacheLock.Unlock()

	if c, ok := b.channelCache[id]; ok {
		return c
	}

	c, err := b.API.GetChannelInfo(id)
	if !OK(err) {
		return nil
	}

	b.channelCache[id] = c
	return c
}

// GetChannelName maps Slack IDs to channel names.
func (b *Bot) GetChannelName(id string) string {
	c := b.GetChannel(id)
	if c == nil {
		return id
	}

	return c.Name
}
