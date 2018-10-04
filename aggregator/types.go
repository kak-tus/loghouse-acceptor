package aggregator

import (
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/kak-tus/loghouse-acceptor/clickhouse"
	"github.com/kak-tus/loghouse-acceptor/request"
	"go.uber.org/zap"
)

// Aggregator object
type Aggregator struct {
	logger  *zap.SugaredLogger
	db      *clickhouse.DB
	decoder jsoniter.API
	config  aggregatorConfig
	C       chan request.Request
	m       *sync.Mutex
}

type requestAgg struct {
	partition string
	args      []interface{}
}

type aggregatorConfig struct {
	PartitionFormat string
	Period          int
	Batch           int
}

const sql = "INSERT INTO logs.logs%s" +
	" (date,timestamp,nsec,host,level,tag,pid,caller,msg," +
	"`string_fields.names`,`string_fields.values`," +
	"`number_fields.names`,`number_fields.values`,`boolean_fields.names`," +
	"`boolean_fields.values`,`null_fields.names`,phone,request_id,order_id," +
	"subscription_id) " +
	"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"
