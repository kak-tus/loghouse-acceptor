package listener

import (
	"sync"

	"github.com/kak-tus/loghouse-acceptor/aggregator"
	"go.uber.org/zap"
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

// Listener holds listener object
type Listener struct {
	logger     *zap.SugaredLogger
	server     *syslog.Server
	aggregator *aggregator.Aggregator
	channel    syslog.LogPartsChannel
	m          *sync.Mutex
}
