FROM 	alpine:latest

RUN	mkdir /app
WORKDIR /app
COPY	bot.linux /app/bot

cmd 	[ "/app/bot" ]


