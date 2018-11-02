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
	logger          *zap.SugaredLogger
	db              *clickhouse.DB
	decoder         jsoniter.API
	config          aggregatorConfig
	C               chan request.Request
	m               *sync.Mutex
	sql             string
	partitionFormat string
}

type requestAgg struct {
	partition string
	args      []interface{}
}

type aggregatorConfig struct {
	PartitionType   string
	PartitionTypes  map[string]string
	Period          int
	Batch           int
	InsertQueryType string
	InsertQueries   map[string]string
}
