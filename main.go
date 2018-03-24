package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kshvakov/clickhouse"
	relp "github.com/stith/gorelp"
)

type reqType struct {
	Time          string `json:"time"`
	Tag           string `json:"tag"`
	Host          string `json:"host"`
	IP            string `json:"ip"`
	Source        string `json:"source"`
	Hostname      string `json:"hostname"`
	Program       string `json:"program"`
	Priority      uint16 `json:"priority"`
	Severity      uint16 `json:"severity"`
	Facility      uint16 `json:"facility"`
	SeverityLabel string `json:"severity_label"`
	FacilityLabel string `json:"facility_label"`
	Msg           string `json:"msg"`
	ToCHtimestamp string
	ToCHnsec      int
	ToCHdate      string
}

var db *sql.DB
var logger = log.New(os.Stdout, "", log.LstdFlags)
var errLogger = log.New(os.Stderr, "", log.LstdFlags)

func main() {
	connectDB()

	ch := make(chan reqType, 1000000)
	stopChan := make(chan int)
	go aggregate(ch, stopChan)

	go listen(ch)
	go healthcheck()

	<-stopChan
	logger.Println("Exit")
}

func connectDB() {
	addr := os.Getenv("CLICKHOUSE_ADDR")

	var err error
	db, err = sql.Open("clickhouse", "tcp://"+addr+"?write_timeout=60")
	if err != nil {
		errLogger.Panicln(err)
	}

	err = db.Ping()
	if err != nil {
		exception, ok := err.(*clickhouse.Exception)
		if ok {
			errLogger.Panicln(fmt.Sprintf("[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace))
		} else {
			errLogger.Panicln(err)
		}
	}

	return
}

func listen(ch chan reqType) {
	relpServer, err := relp.NewServer("0.0.0.0", 3333, true)
	if err != nil {
		errLogger.Panicln(err)
	}

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case message := <-relpServer.MessageChannel:
			var parsed reqType

			err = json.Unmarshal([]byte(message.Data), &parsed)
			if err != nil {
				errLogger.Println(err, ":", message.Data)
				continue
			}

			ch <- parsed
		case <-stopSignal:
			logger.Println("Got stop signal, stop listening RELP")
			relpServer.Close()

			logger.Println("Close channel")
			close(ch)

			return
		}
	}
}

func aggregate(ch chan reqType, stopChan chan int) {
	period, err := strconv.ParseUint(os.Getenv("ACC_PERIOD"), 10, 64)
	if err != nil {
		errLogger.Println(err)
		period = 60
	}

	batch, err := strconv.ParseUint(os.Getenv("ACC_BATCH"), 10, 64)
	if err != nil {
		errLogger.Println(err)
		batch = 10000
	}

	vals := make([]reqType, batch)
	count := 0

	start := time.Now()

	for {
		message, more := <-ch

		if more {
			count++
			vals[count-1] = message

			if count >= int(batch) {
				send(vals[0:count])
				logger.Println(fmt.Sprintf("Sended %d values", count))
				count = 0
			}
		}

		if time.Now().Sub(start).Seconds() >= float64(period) || !more {
			send(vals[0:count])
			logger.Println(fmt.Sprintf("Sended %d values", count))
			count = 0

			start = time.Now()
		}

		if !more {
			logger.Println("No more messages")
			stopChan <- 1
			break
		}
	}
}

func send(vals []reqType) {
	byDate := make(map[string][]reqType)

	for i := 0; i < len(vals); i++ {
		dt, err := time.Parse(time.RFC3339, vals[i].Time)
		if err != nil {
			errLogger.Println("RELP date parse failed: ", err)
			continue
		}

		dt = dt.In(time.UTC)

		vals[i].ToCHdate = dt.Format("2006-01-02")
		vals[i].ToCHtimestamp = dt.Format("2006-01-02 15:04:05")
		vals[i].ToCHnsec = dt.Nanosecond()

		byDate[dt.Format("20060102")] = append(byDate[dt.Format("20060102")], vals[i])
	}

	for dt, dtVals := range byDate {
		tx, err := db.Begin()
		if err != nil {
			errLogger.Println(err)
			continue
		}

		sql := "INSERT INTO logs.logs" + dt +
			" (date,timestamp,nsec,source,namespace,host,pod_name,container_name,stream," +
			"`labels.names`,`labels.values`,`string_fields.names`,`string_fields.values`," +
			"`number_fields.names`,`number_fields.values`,`boolean_fields.names`," +
			"`boolean_fields.values`,`null_fields.names`,phone,request_id,order_id," +
			"subscription_id) " +
			"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"

		stmt, err := tx.Prepare(sql)
		if err != nil {
			tx.Rollback()
			errLogger.Println(err)
			continue
		}

		for _, val := range dtVals {
			labelsNames := [0]string{}
			labelsValues := [0]string{}
			stringNames := clickhouse.Array([]string{"msg"})
			stringValues := clickhouse.Array([]string{val.Msg})
			numbersNames := [0]string{}
			numbersValues := [0]int{}
			boolNames := [0]string{}
			boolValues := [0]int{}
			nullNames := [0]string{}

			_, err := stmt.Exec(val.ToCHdate, val.ToCHtimestamp, val.ToCHnsec, val.Source,
				val.FacilityLabel, val.Hostname, val.Tag, val.Program, val.SeverityLabel,
				labelsNames, labelsValues, stringNames, stringValues, numbersNames,
				numbersValues, boolNames, boolValues, nullNames, 79031234567,
				"", "", "")

			if err != nil {
				tx.Rollback()
				tx = nil
				errLogger.Println(err)
				break
			}
		}

		if tx == nil {
			continue
		}

		err = tx.Commit()
		if err != nil {
			errLogger.Println(err)
			return
		}
	}
}

func healthcheck() {
	srv := &http.Server{Addr: ":9001"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		err := db.Ping()
		if err != nil {
			errLogger.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "err")
			return
		}

		fmt.Fprintf(w, "ok")
	})

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stopSignal
		logger.Println("Got stop signal, stop listening HTTP")

		err := srv.Shutdown(nil)
		if err != nil {
			errLogger.Println(err)
		}
	}()

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		errLogger.Panicln(err)
	}
}
