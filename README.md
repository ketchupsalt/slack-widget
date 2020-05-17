# Trivial Slack bot

See `slack.go` for Slack API; `cmd/bot/main.go` is the bot itself. It's tiny and doesn't do anything.

To deploy: 

1. Create a new Slack app for your workspace; give it `channels:{history,read}`, `chat:write` and `users.read`.
   Install it; leave the browser window open for a sec.

2. `flyctl apps create`

3. Fix `fly.toml` to set internal_port to 3000

4. `flyctl secrets set SLACK_XOXB=[your xoxb here]`

5. `make deploy` (the Makefile and Dockerfile are both trivial)

6. `flyctl info`

7. Back in the Slack app window, go to the "Event Subscriptions" tab, add the URL for the Fly app
   (it's the Fly hostname + "/events-endpoint"); it should verify. Subscribe to `messages.channels`.
   
8. In actual Slack, invite your bot to a channel; it should now see messages and annoy you with responses.


