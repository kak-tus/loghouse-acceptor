package aggregator

import (
	"fmt"
	"sync"
	"time"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/iph0/conf"
	jsoniter "github.com/json-iterator/go"
	"github.com/kak-tus/loghouse-acceptor/clickhouse"
	"github.com/kak-tus/loghouse-acceptor/request"
)

var aggregatorObj *Aggregator

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["aggregator"]

			var cnf aggregatorConfig
			err := conf.Decode(cnfMap, &cnf)
			if err != nil {
				return err
			}

			aggregatorObj = &Aggregator{
				logger:  applog.GetLogger().Sugar(),
				db:      clickhouse.GetDB(),
				decoder: jsoniter.Config{UseNumber: true}.Froze(),
				config:  cnf,
				C:       make(chan request.Request, 1000000),
				m:       &sync.Mutex{},
			}

			aggregatorObj.logger.Info("Inited aggregator")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			aggregatorObj.logger.Info("Stop aggregator")
			aggregatorObj.m.Lock()
			aggregatorObj.logger.Info("Stopped aggregator")
			return nil
		},
	)
}

// GetAggregator returns object
func GetAggregator() *Aggregator {
	return aggregatorObj
}

// Start aggregation
func (a *Aggregator) Start() {
	go a.aggregate()
	go a.db.CreatePartitions(a.config.PartitionFormat)
}

// Stop aggregation
func (a *Aggregator) Stop() {
	close(a.C)
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

		if time.Now().Sub(start).Seconds() >= float64(a.config.Period) {
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
		prepared := fmt.Sprintf(sql, vals[i].partition)
		byDate[sql] = append(byDate[prepared], vals[i].args)
	}

	errors := a.db.Send(byDate)
	if errors != nil {
		for _, err := range errors {
			a.logger.Error(err)
		}
	}

	// Clean
	for sql := range byDate {
		byDate[sql] = nil
		delete(byDate, sql)
	}
}
