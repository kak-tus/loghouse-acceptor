package aggregator

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/kak-tus/loghouse-acceptor/clickhouse"
	"github.com/kak-tus/loghouse-acceptor/listener"
)

// Aggregator object
type Aggregator struct {
	ch        chan listener.Request
	logger    *log.Logger
	errLogger *log.Logger
	db        *clickhouse.DB
	decoder   jsoniter.API
}

type request struct {
	partition string
	args      []interface{}
}

const partitionFormat = "2006010215"

// New returns new object
func New(ch chan listener.Request) (*Aggregator, error) {
	db, err := clickhouse.Get()
	if err != nil {
		return nil, err
	}

	a := &Aggregator{
		ch:        ch,
		logger:    log.New(os.Stdout, "", 0),
		errLogger: log.New(os.Stderr, "", 0),
		db:        db,
		decoder:   jsoniter.Config{UseNumber: true}.Froze(),
	}

	return a, nil
}

// Aggregate start aggregation
func (a Aggregator) Aggregate() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		a.aggregate()
		wg.Done()
	}()

	go func() {
		a.db.CreatePartitions(partitionFormat)
		wg.Done()
	}()

	wg.Wait()
}

func (a Aggregator) aggregate() {
	period, err := strconv.ParseUint(os.Getenv("ACC_PERIOD"), 10, 64)
	if err != nil {
		a.errLogger.Println(err)
		period = 60
	}

	batch, err := strconv.ParseUint(os.Getenv("ACC_BATCH"), 10, 64)
	if err != nil {
		a.errLogger.Println(err)
		batch = 10000
	}

	vals := make([]request, batch)
	count := 0

	start := time.Now()

	for {
		req, more := <-a.ch

		if more {
			vals[count] = a.convert(req)
			count++
		}

		if count >= int(batch) {
			a.send(vals[0:count])
			a.logger.Println(fmt.Sprintf("Sended %d values", count))
			count = 0
			start = time.Now()
		}

		if time.Now().Sub(start).Seconds() >= float64(period) {
			a.send(vals[0:count])
			a.logger.Println(fmt.Sprintf("Sended %d values", count))
			count = 0
			start = time.Now()
		}

		if !more {
			a.send(vals[0:count])
			a.logger.Println(fmt.Sprintf("Sended %d values", count))

			a.logger.Println("No more messages")
			break
		}
	}
}

func (a Aggregator) convert(req listener.Request) request {
	var res request

	dt := req.Time.In(time.UTC)

	res.partition = dt.Format(partitionFormat)

	args := []interface{}{
		dt.Format("2006-01-02"),
		dt.Format("2006-01-02 15:04:05"),
		dt.Nanosecond(),
		req.Hostname,
	}

	parsed := a.parse(req)
	args = append(args, parsed...)

	res.args = args

	return res
}

func (a Aggregator) send(vals []request) {
	byDate := make(map[string][]interface{})

	for i := 0; i < len(vals); i++ {
		sql := "INSERT INTO logs.logs" + vals[i].partition +
			" (date,timestamp,nsec,host,level,tag,pid,caller,msg," +
			"`string_fields.names`,`string_fields.values`," +
			"`number_fields.names`,`number_fields.values`,`boolean_fields.names`," +
			"`boolean_fields.values`,`null_fields.names`,phone,request_id,order_id," +
			"subscription_id) " +
			"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"

		byDate[sql] = append(byDate[sql], vals[i].args)
	}

	errors := a.db.Send(byDate)
	if errors != nil {
		for _, err := range errors {
			a.logger.Println(err)
		}
	}

	// Clean
	for sql := range byDate {
		byDate[sql] = nil
		delete(byDate, sql)
	}
}
