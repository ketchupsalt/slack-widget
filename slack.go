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

func OK(err error) bool {
	if err != nil {
		log.Printf("unexpected error: %s", err)
		return false
	}

	return true
}

type Bot struct {
	API    *slack.Client
	User   string
	Events chan slackevents.EventsAPIInnerEvent

	listener     *http.Server
	userCache    map[string]*slack.User
	channelCache map[string]*slack.Channel
	cacheLock    sync.Mutex
}

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

func (b *Bot) Stop() {
	b.listener.Shutdown(context.TODO())
}

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

func (b *Bot) GetUserName(id string) string {
	u := b.GetUser(id)
	if u == nil {
		return id
	}
	return u.Name
}

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

func (b *Bot) GetChannelName(id string) string {
	c := b.GetChannel(id)
	if c == nil {
		return id
	}

	return c.Name
}
