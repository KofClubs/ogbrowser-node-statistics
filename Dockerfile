# Build from alpine based golang environment
FROM golang:1.12-alpine as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git

ENV GOPROXY https://goproxy.io
ENV GO111MODULE on

WORKDIR /opt/go/
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o statistics ./server

FROM alpine:latest

RUN apk add --no-cache curl iotop busybox-extras

COPY --from=builder /opt/go/ /opt/go/

EXPOSE 8080

WORKDIR /opt/go

CMD ["./statistics"]
