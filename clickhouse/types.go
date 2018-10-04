package clickhouse

import (
	"database/sql"

	"go.uber.org/zap"
)

// DB handle DB connection object
type DB struct {
	DB     *sql.DB
	logger *zap.SugaredLogger
}

type clickhouseConfig struct {
	Addr string
}

const sqlTable = "CREATE TABLE IF NOT EXISTS logs.logs%s" +
	" ON CLUSTER so" +
	" ( date Date, timestamp DateTime, nsec UInt32, namespace String," +
	" level String, tag String, host String, pid String, caller String," +
	" msg String, labels Nested ( names String, values String )," +
	" string_fields Nested ( names String, values String )," +
	" number_fields Nested ( names String, values Float64 )," +
	" boolean_fields Nested ( names String, values UInt8 )," +
	" `null_fields.names` Array(String), phone UInt64, request_id String," +
	" order_id String, subscription_id String )" +
	" ENGINE = Distributed( 'so', 'logs', 'logs%s" +
	"_shard', rand() );"

const sqlShard = "CREATE TABLE IF NOT EXISTS logs.logs%s" +
	"_shard ON CLUSTER so" +
	" ( date Date, timestamp DateTime, nsec UInt32, namespace String," +
	" level String, tag String, host String, pid String, caller String," +
	" msg String, labels Nested ( names String, values String )," +
	" string_fields Nested ( names String, values String )," +
	" number_fields Nested ( names String, values Float64 )," +
	" boolean_fields Nested ( names String, values UInt8 )," +
	" `null_fields.names` Array(String), phone UInt64, request_id String," +
	" order_id String, subscription_id String )" +
	" ENGINE = ReplicatedMergeTree( " +
	"'/clickhouse/tables/{shard}/logs_logs%s" +
	"_shard', '{replica}'," +
	" date, ( timestamp, nsec, level, tag, host," +
	" phone, request_id, order_id, subscription_id ), 32768 );"
