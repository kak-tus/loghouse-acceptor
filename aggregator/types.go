package aggregator

import (
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/kak-tus/loghouse-acceptor/clickhouse"
	"github.com/kak-tus/loghouse-acceptor/config"
	"github.com/kak-tus/loghouse-acceptor/request"
	"go.uber.org/zap"
)

// Aggregator object
type Aggregator struct {
	logger          *zap.SugaredLogger
	db              *clickhouse.DB
	decoder         jsoniter.API
	config          config.AggregatorConfig
	C               chan request.Request
	m               *sync.Mutex
	sql             string
	partitionFormat string
}

type requestAgg struct {
	partition string
	args      []interface{}
}
