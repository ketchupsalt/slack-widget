
all: bot


bot: slack.go cmd/bot/main.go
	go build -o ./bot ./cmd/bot


bot.linux: slack.go cmd/bot/main.go
	GOOS=linux GOARCH=amd64 go build -o ./bot.linux ./cmd/bot


.PHONY: docker
docker: Dockerfile bot.linux
	docker build -t bot .


.PHONE: deploy
deploy: bot.linux
	flyctl deploy

