# loghouse-acceptor

Accept and write logs to ClickHouse in Loghouse compatible format.

Daemon accepts log only in syslog RFC5424 format from rsyslog. Later may be will be added
some additinal protocols support.

## Daemon configuration

Configuration is possible with environment variables

### CLICKHOUSE_ADDR

ClickHouse address

```
CLICKHOUSE_ADDR=127.0.0.1:9000
```

### ACC_PERIOD

Period in seconds to send data to ClickHouse if batch size is not reached.

```
ACC_PERIOD=60
```

### ACC_BATCH

Batch size to send to ClickHouse immediately.

```
ACC_BATCH=10000
```

### ACC_TABLE_TYPE

Loghouse table type. In most cases you need basic table type - default to Loghouse. But you can use other type - extended, with some additional vendored columns.

```
ACC_TABLE_TYPE=basic
```

### ACC_PARTITION_TYPE

Configurable partitions periods - daily or hourly. Loghouse supports both types.

```
ACC_PARTITION_TYPE=hourly
```

### ACC_SHARD_TYPE

Sharding type. If you didn't use shards and replicas set this to default value "basic". In other case see "Sharding and replication in ClickHouse" section.

```
ACC_SHARD_TYPE=basic
```

### ACC_TABLE_CLUSTER

Cluster name in ClickHouse. See "Sharding and replication in ClickHouse" section.

```
ACC_TABLE_CLUSTER=my-cluster
```

## Rsyslog configuration

Configure rsyslog to resend logs to daemon

```
action(type="omfwd" Target="127.0.0.1" Port="3333" Protocol="tcp" KeepAlive="on" Template="RSYSLOG_SyslogProtocol23Format")
```

## Running

```
docker run --rm -it -p 3333:3333 -e ClickHouse_ADDR=127.0.0.1:9000 kaktuss/loghouse-acceptor
```


## Sharding and replication in ClickHouse

It is possible to use and create sharded and replicated partitions. See [replication](https://ClickHouse.yandex/docs/en/operations/table_engines/replication/) and [distributed](https://ClickHouse.yandex/docs/en/operations/table_engines/distributed/#table_engines-distributed) in ClickHouse docs.

To use this feature with loghouse-acceptor you need to configure two macros in ClickHouse.

```
<macros>
    ...
    <shard>02</shard>
    <replica>example05-02-1.yandex.ru</replica>
    ...
</macros>
```

Then configure cluster in ClickHouse

```
<remote_servers>
  <my-cluster>
    ...
  </my-cluster>
<remote_servers>
```

Then set ACC_SHARD_TYPE variable to basic_sharded and ACC_TABLE_CLUSTER to your cluster name.

```
ACC_SHARD_TYPE=basic_sharded
ACC_TABLE_CLUSTER=my-cluster
```
