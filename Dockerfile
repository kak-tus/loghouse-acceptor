FROM golang:1.10-alpine AS build

WORKDIR /go/src/github.com/kak-tus/loghouse-acceptor
COPY main.go ./
COPY clickhouse ./
COPY healthcheck ./
RUN apk add --no-cache git && go build

FROM alpine:3.7

RUN \
  apk add --no-cache \
    su-exec \
    tzdata

COPY --from=build /go/bin/loghouse-acceptor /usr/local/bin/loghouse-acceptor
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

ENV \
  USER_UID=1000 \
  USER_GID=1000 \
  \
  CLICKHOUSE_ADDR= \
  ACC_PERIOD=60 \
  ACC_BATCH=10000

EXPOSE 3333

CMD ["/usr/local/bin/entrypoint.sh"]
