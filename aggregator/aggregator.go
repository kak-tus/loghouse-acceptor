package aggregator

import (
	"errors"
	"fmt"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/kak-tus/loghouse-acceptor/clickhouse"
	"github.com/kak-tus/loghouse-acceptor/config"
	"github.com/kak-tus/loghouse-acceptor/request"
	"go.uber.org/zap"
)

func NewAggregator(cnfFull *config.Config, log *zap.SugaredLogger) (*Aggregator, error) {
	cnf := cnfFull.Aggregator

	sql, ok := cnf.InsertQueries[cnf.InsertQueryType]
	if !ok {
		return nil, errors.New("Unsupported table type: " + cnf.InsertQueryType)
	}

	pfmt, ok := cnf.PartitionTypes[cnf.PartitionType]
	if !ok {
		return nil, errors.New("Unsupported partition type: " + cnf.PartitionType)
	}

	ch, err := clickhouse.NewDB(cnfFull.Clickhouse, log)
	if err != nil {
		return nil, err
	}

	aggregatorObj := &Aggregator{
		logger:          log,
		db:              ch,
		decoder:         jsoniter.Config{UseNumber: true}.Froze(),
		config:          cnf,
		C:               make(chan request.Request, 1000000),
		m:               &sync.Mutex{},
		sql:             sql,
		partitionFormat: pfmt,
	}

	return aggregatorObj, nil
}

// Start aggregation
func (a *Aggregator) Start() {
	go a.aggregate()
	go a.db.CreatePartitions(a.partitionFormat, a.config.PartitionType)
	a.logger.Info("Inited aggregator")
}

// Stop aggregation
func (a *Aggregator) Stop() {
	a.logger.Info("Stop aggregator")
	close(a.C)
	a.db.Stop()
	a.m.Lock()
	a.logger.Info("Stopped aggregator")
}

func (a *Aggregator) aggregate() {
	a.m.Lock()

	vals := make([]requestAgg, a.config.Batch)
	count := 0

	start := time.Now()

	for {
		req, more := <-a.C

		if more {
			vals[count] = a.convert(req)
			count++
		}

		if count >= int(a.config.Batch) {
			a.send(vals[0:count])
			a.logger.Infof("Sended %d values", count)
			count = 0
			start = time.Now()
		}

		if time.Since(start).Seconds() >= float64(a.config.Period) {
			a.send(vals[0:count])
			a.logger.Infof("Sended %d values", count)
			count = 0
			start = time.Now()
		}

		if !more {
			a.send(vals[0:count])
			a.logger.Infof("Sended %d values", count)

			a.logger.Info("No more messages")
			break
		}
	}

	a.m.Unlock()
}

func (a *Aggregator) send(vals []requestAgg) {
	byDate := make(map[string][][]interface{})

	for i := 0; i < len(vals); i++ {
		prepared := fmt.Sprintf(a.sql, vals[i].partition)
		byDate[prepared] = append(byDate[prepared], vals[i].args)
	}

	errors := a.db.Send(byDate)
	for _, err := range errors {
		a.logger.Error(err)
	}

	// Clean
	for sql := range byDate {
		byDate[sql] = nil
		delete(byDate, sql)
	}
}
