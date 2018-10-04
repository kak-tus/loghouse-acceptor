FROM golang:1.10.3-alpine3.8 AS build

WORKDIR /go/src/github.com/kak-tus/loghouse-acceptor

COPY aggregator ./aggregator/
COPY clickhouse ./clickhouse/
COPY main.go ./
COPY listener ./listener/
COPY request ./request/
COPY vendor ./vendor/

RUN go install

FROM alpine:3.8

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
