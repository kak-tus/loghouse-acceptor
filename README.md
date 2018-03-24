# loghouse-acceptor

Accept and write logs to Clickhouse in Loghouse compatible format.

Daemon accepts log only in RELP format from rsyslog. Later may be will be added
some additinal protocols support.

## Configuration

Environment variables

Clickhouse address

CLICKHOUSE_ADDR=127.0.0.1:9000

Period in seconds to send data to Clickhouse

ACC_PERIOD=60

Batch size to send to Clickhouse immediately

ACC_BATCH=10000

## Running

docker run --rm -it -p 3333:3333 -e CLICKHOUSE_ADDR=cloud-11.dd:9000 kaktuss/loghouse-acceptor
