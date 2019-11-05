FROM golang:1.13.1-alpine3.10 AS build

WORKDIR /go/loghouse-acceptor

COPY aggregator ./aggregator
COPY clickhouse ./clickhouse
COPY config ./config
COPY go.mod .
COPY go.sum .
COPY listener ./listener
COPY main.go .
COPY request ./request

RUN go build -o /go/bin/loghouse-acceptor

FROM alpine:3.10

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
