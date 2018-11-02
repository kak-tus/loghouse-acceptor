FROM golang:1.10.3-alpine3.8 AS build

WORKDIR /go/src/github.com/kak-tus/loghouse-acceptor

COPY aggregator ./aggregator/
COPY clickhouse ./clickhouse/
COPY listener ./listener/
COPY request ./request/
COPY vendor ./vendor/
COPY main.go ./

RUN go install

FROM alpine:3.8

RUN \
  apk add --no-cache \
    tzdata \
  && adduser -DH user

COPY --from=build /go/bin/loghouse-acceptor /usr/local/bin/loghouse-acceptor
COPY etc /etc/

ENV \
  CLICKHOUSE_ADDR= \
  ACC_PERIOD=60 \
  ACC_BATCH=10000 \
  ACC_TABLE_TYPE=basic \
  ACC_PARTITION_TYPE=hourly \
  ACC_SHARD_TYPE=basic \
  ACC_TABLE_CLUSTER=

EXPOSE 3333 9000

USER user

CMD ["/usr/local/bin/loghouse-acceptor"]
