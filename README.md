# loghouse-acceptor

Accept and write logs to Clickhouse in Loghouse compatible format.

Daemon accepts log only in syslog RFC5424 format from rsyslog. Later may be will be added
some additinal protocols support.

## Configuration

Environment variables

Clickhouse address

CLICKHOUSE_ADDR=127.0.0.1:9000

Period in seconds to send data to Clickhouse

ACC_PERIOD=60

Batch size to send to Clickhouse immediately

ACC_BATCH=10000

Configure rsyslog to resend logs to daemon

```
action(type="omfwd" Target="127.0.0.1" Port="3333" Protocol="tcp" KeepAlive="on" Template="RSYSLOG_SyslogProtocol23Format")
```

## Running

docker run --rm -it -p 3333:3333 -e CLICKHOUSE_ADDR=127.0.0.1:9000 kaktuss/loghouse-acceptor
