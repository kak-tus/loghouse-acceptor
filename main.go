package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kshvakov/clickhouse"
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

type reqType struct {
	Time          string `json:"time"`
	Tag           string `json:"tag"`
	Hostname      string `json:"hostname"`
	Program       string `json:"program"`
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

	go listenSyslog(ch)
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

func listenSyslog(ch chan reqType) {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)

	err := server.ListenTCP("0.0.0.0:3333")
	if err != nil {
		errLogger.Panicln(err)
	}

	server.Boot()

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	severityLabels := map[int]string{0: "emerg", 1: "alert", 2: "crit", 3: "error",
		4: "warning", 5: "notice", 6: "info", 7: "debug"}
	facilityLabels := map[int]string{0: "kern", 1: "user", 2: "mail", 3: "daemon",
		4: "auth", 5: "syslog", 6: "lpr", 7: "news", 8: "uucp", 9: "cron", 10: "security",
		11: "ftp", 12: "ntp", 13: "logaudit", 14: "logalert", 15: "clock", 16: "local0",
		17: "local1", 18: "local2", 19: "local3", 20: "local4", 21: "local5",
		22: "local6", 23: "local7"}

	go func(channel syslog.LogPartsChannel) {
		for {
			select {
			case message := <-channel:
				parsed := reqType{
					Time:          message["timestamp"].(time.Time).Format(time.RFC3339),
					Tag:           message["proc_id"].(string),
					Hostname:      message["hostname"].(string),
					Program:       message["app_name"].(string),
					SeverityLabel: severityLabels[message["severity"].(int)],
					FacilityLabel: facilityLabels[message["facility"].(int)],
					Msg:           strings.TrimLeft(message["message"].(string), " ")}

				ch <- parsed
			case <-stopSignal:
				logger.Println("Got stop signal, stop listening Syslog")
				server.Kill()

				logger.Println("Close channel")
				close(ch)

				return
			}
		}
	}(channel)

	server.Wait()
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
			errLogger.Println("RFC3339 date parse failed: ", err)
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

			_, err := stmt.Exec(val.ToCHdate, val.ToCHtimestamp, val.ToCHnsec, "",
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
