# Build from alpine based golang environment
FROM golang:1.13-alpine as builder

RUN apk add musl-dev linux-headers git gcc

ENV GOPROXY https://goproxy.io
ENV GO111MODULE on

WORKDIR /opt/go/
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o ./build/statistics ./server

FROM alpine:latest

RUN apk add --no-cache curl iotop busybox-extras

COPY --from=builder /opt/go/ /opt/go/

EXPOSE 8080

WORKDIR /opt/go/build

CMD ["./statistics"]
