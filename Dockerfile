FROM golang:latest

ENV GO111MODULE on
ENV GOPROXY https://goproxy.io

CMD [go build ./server/main.go]