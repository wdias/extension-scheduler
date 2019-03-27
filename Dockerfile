FROM golang:1.12.0-alpine

WORKDIR /go/src/app
RUN apk update && apk add git
RUN go get -u github.com/kataras/iris
RUN go get -u github.com/robfig/cron
RUN go get -u github.com/go-redis/redis
COPY ./src .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["app"]
