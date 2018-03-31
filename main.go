package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/json-iterator/go"
	"github.com/kshvakov/clickhouse"
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

type reqType struct {
	Time          time.Time
	Level         string
	Tag           string
	Pid           string
	Hostname      string
	Facility      string
	Msg           string
	ToCHtimestamp string
	ToCHnsec      int
	ToCHdate      string
}

var db *sql.DB
var logger = log.New(os.Stdout, "", log.LstdFlags)
var errLogger = log.New(os.Stderr, "", log.LstdFlags)

const partitionFormat = "2006010215"

var decoder jsoniter.API

func main() {
	db = connectDB()

	decoder = jsoniter.Config{UseNumber: true}.Froze()

	ch := make(chan reqType, 1000000)
	stopChan := make(chan int)
	go aggregate(ch, stopChan)

	go listenSyslog(ch)
	go listenHealthcheck()

	go createPartitions()

	<-stopChan
	logger.Println("Exit")
}

func connectDB() *sql.DB {
	addr := os.Getenv("CLICKHOUSE_ADDR")

	db, err := sql.Open("clickhouse", "tcp://"+addr+"?write_timeout=60")
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

	return db
}

func listenSyslog(ch chan reqType) {
	channel := make(syslog.LogPartsChannel, 100000)
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

	levels := map[int]string{0: "FATAL", 1: "FATAL", 2: "FATAL", 3: "ERROR",
		4: "WARN", 5: "INFO", 6: "INFO", 7: "DEBUG"}
	facilities := map[int]string{0: "kern", 1: "user", 2: "mail", 3: "daemon",
		4: "auth", 5: "syslog", 6: "lpr", 7: "news", 8: "uucp", 9: "cron", 10: "security",
		11: "ftp", 12: "ntp", 13: "logaudit", 14: "logalert", 15: "clock", 16: "local0",
		17: "local1", 18: "local2", 19: "local3", 20: "local4", 21: "local5",
		22: "local6", 23: "local7"}

	go func(channel syslog.LogPartsChannel) {
		for {
			select {
			case message := <-channel:
				parsed := reqType{
					Time:     message["timestamp"].(time.Time),
					Level:    levels[message["severity"].(int)],
					Tag:      message["app_name"].(string),
					Pid:      message["proc_id"].(string),
					Hostname: message["hostname"].(string),
					Facility: facilities[message["facility"].(int)],
					Msg:      strings.Trim(message["message"].(string), " ")}

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
		}

		if count >= int(batch) {
			send(vals[0:count])
			logger.Println(fmt.Sprintf("Sended %d values", count))
			count = 0
			start = time.Now()
		}

		if time.Now().Sub(start).Seconds() >= float64(period) {
			send(vals[0:count])
			logger.Println(fmt.Sprintf("Sended %d values", count))
			count = 0
			start = time.Now()
		}

		if !more {
			send(vals[0:count])
			logger.Println(fmt.Sprintf("Sended %d values", count))

			logger.Println("No more messages")
			stopChan <- 1
			break
		}
	}
}

func send(vals []reqType) {
	byDate := make(map[string][]reqType)

	for i := 0; i < len(vals); i++ {
		dt := vals[i].Time.In(time.UTC)

		vals[i].ToCHdate = dt.Format("2006-01-02")
		vals[i].ToCHtimestamp = dt.Format("2006-01-02 15:04:05")
		vals[i].ToCHnsec = dt.Nanosecond()

		byDate[dt.Format(partitionFormat)] = append(byDate[dt.Format(partitionFormat)], vals[i])
	}

	for dt, dtVals := range byDate {
		tx, err := db.Begin()
		if err != nil {
			errLogger.Println(err)
			continue
		}

		sql := "INSERT INTO logs.logs" + dt +
			" (date,timestamp,nsec,host,level,tag,pid,caller,msg," +
			"`string_fields.names`,`string_fields.values`," +
			"`number_fields.names`,`number_fields.values`,`boolean_fields.names`," +
			"`boolean_fields.values`,`null_fields.names`,phone,request_id,order_id," +
			"subscription_id) " +
			"VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);"

		stmt, err := tx.Prepare(sql)
		if err != nil {
			tx.Rollback()
			errLogger.Println(err)
			continue
		}

		for _, val := range dtVals {
			args := []interface{}{
				val.ToCHdate,
				val.ToCHtimestamp,
				val.ToCHnsec,
				val.Hostname}

			parsed := parse(val)
			args = append(args, parsed...)

			_, err := stmt.Exec(args...)

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

	// Clean
	for dt := range byDate {
		byDate[dt] = nil
		delete(byDate, dt)
	}
}

func listenHealthcheck() {
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

func parse(val reqType) []interface{} {
	var jsons []interface{}

	str := val.Msg

	// Full json string
	if strings.Index(str, "{") == 0 && str[len(str)-1:] == "}" {
		var parsed interface{}
		err := decoder.UnmarshalFromString(str, &parsed)
		if err == nil {
			jsons = append(jsons, parsed)
		}
	} else if strings.Index(str, "{") >= 0 {
		// Special vendor-locked case
		if strings.Index(str, " c{") >= 0 && str[len(str)-1:] == "}" {
			from := strings.Index(str, " c{")

			var parsed interface{}
			err := decoder.UnmarshalFromString(str[from+2:], &parsed)
			if err == nil {
				jsons = append(jsons, parsed)
				str = str[:from]
			}
		}

		if strings.Index(str, "{") >= 0 && strings.LastIndex(str, "}") >= 0 {
			from := strings.Index(str, "{")
			to := strings.LastIndex(str, "}")

			var parsed interface{}
			err := decoder.UnmarshalFromString(str[from:to+1], &parsed)
			if err == nil {
				jsons = append(jsons, parsed)

				// Special vendor-locked case
				if str[from-1:from] == "j" {
					str = str[:from-1] + str[to+1:]
				} else {
					str = str[:from] + str[to+1:]
				}
			}
		}
	}

	var stringNames []string
	var stringVals []string
	var boolNames []string
	var boolVals []uint8
	var numNames []string
	var numVals []float64
	var nullNames []string

	phone := 0
	requestID := ""
	orderID := ""
	subscriptionID := ""
	caller := ""

	level := val.Level
	tag := val.Tag
	pid := val.Pid

	if len(jsons) > 0 {
		for _, js := range jsons {
			mapped := js.(map[string]interface{})

			for key, val := range mapped {
				// Some vendor-locked logic
				if key == "phone" {
					switch val.(type) {
					case string:
						conv, err := strconv.Atoi(val.(string))
						if err == nil {
							phone = conv
							continue
						}
					case json.Number:
						conv, err := strconv.Atoi(string(val.(json.Number)))
						if err == nil {
							phone = conv
							continue
						}
					}
				} else if key == "request_id" {
					switch val.(type) {
					case string:
						requestID = val.(string)
						continue
					}
				} else if key == "order_id" {
					switch val.(type) {
					case string:
						orderID = val.(string)
						continue
					}
				} else if key == "subscription_id" {
					switch val.(type) {
					case string:
						subscriptionID = val.(string)
						continue
					}
				} else if key == "level" {
					switch val.(type) {
					case string:
						conv := val.(string)

						if conv == "DEBUG" || conv == "INFO" ||
							conv == "WARN" || conv == "ERROR" ||
							conv == "FATAL" {
							level = conv
						} else if conv == "TRACE" {
							level = "DEBUG"
						} else if conv == "PANIC" {
							level = "FATAL"
						} else {
							level = "DEBUG"
						}

						continue
					}
				} else if key == "tag" {
					switch val.(type) {
					case string:
						tag = val.(string)
						continue
					}
				} else if key == "pid" {
					switch val.(type) {
					case string:
						pid = val.(string)
						continue
					case json.Number:
						pid = string(val.(json.Number))
						continue
					}
				} else if key == "caller" {
					switch val.(type) {
					case string:
						caller = val.(string)
						continue
					}
				} else if key == "id" {
					_, ok := mapped["request_id"]
					if ok {
						switch val.(type) {
						case string:
							requestID = val.(string)
							continue
						}
					}
				}

				switch val.(type) {
				case string:
					stringNames = append(stringNames, key)
					stringVals = append(stringVals, val.(string))
				case bool:
					var conv uint8
					if val.(bool) {
						conv = 1
					} else {
						conv = 0
					}

					boolNames = append(boolNames, key)
					boolVals = append(boolVals, conv)
				case json.Number:
					if strings.Index(string(val.(json.Number)), ".") >= 0 {
						conv, err := strconv.ParseFloat(string(val.(json.Number)), 64)
						if err == nil {
							numNames = append(numNames, key)
							numVals = append(numVals, conv)
						}
					} else {
						conv, err := strconv.Atoi(string(val.(json.Number)))
						if err == nil {
							numNames = append(numNames, key)
							numVals = append(numVals, float64(conv))
						}
					}
				case nil:
					nullNames = append(nullNames, key)
				default:
					conv, err := decoder.Marshal(val)
					if err == nil {
						stringNames = append(stringNames, key)
						stringVals = append(stringVals, string(conv))
					}
				}
			}
		}
	}

	var res []interface{}

	res = append(res, level, tag, pid, caller, str,
		clickhouse.Array(stringNames), clickhouse.Array(stringVals))

	if len(numNames) > 0 {
		res = append(res, clickhouse.Array(numNames), clickhouse.Array(numVals))
	} else {
		res = append(res, [0]string{}, [0]float64{})
	}

	if len(boolNames) > 0 {
		res = append(res, clickhouse.Array(boolNames), clickhouse.Array(boolVals))
	} else {
		res = append(res, [0]string{}, [0]int{})
	}

	if len(nullNames) > 0 {
		res = append(res, clickhouse.Array(nullNames))
	} else {
		res = append(res, [0]int{})
	}

	res = append(res, phone, requestID, orderID, subscriptionID)

	return res
}

func createPartitions() {
	ticker := time.NewTicker(time.Hour + time.Second*time.Duration(rand.Intn(100)))

	for {
		<-ticker.C
		logger.Println("Check partitions")

		// Start create partitions from some times ago
		// to allow recreate partitions in case of stopped daemon for some time
		dt := time.Now().Add(-time.Hour * 24)

		for i := 1; i <= 48; i++ {
			partition := dt.Format(partitionFormat)
			dt = dt.Add(time.Hour)

			sql := "SELECT 1 FROM system.tables WHERE database = 'logs'	AND name = 'logs" +
				partition + "';"

			rows, err := db.Query(sql)
			if err != nil {
				errLogger.Println(err)
				continue
			}

			if rows.Next() {
				rows.Close()
				continue
			}

			rows.Close()
			logger.Println("Create partition " + partition)

			sql = "CREATE TABLE logs.logs" + partition +
				" ( date Date, timestamp DateTime, nsec UInt32, namespace String," +
				" level String, tag String, host String, pid String, caller String," +
				" msg String, labels Nested ( names String, values String )," +
				" string_fields Nested ( names String, values String )," +
				" number_fields Nested ( names String, values Float64 )," +
				" boolean_fields Nested ( names String, values UInt8 )," +
				" `null_fields.names` Array(String), phone UInt64, request_id String," +
				" order_id String, subscription_id String )" +
				" ENGINE = MergeTree( date, ( timestamp, nsec, level, tag, host," +
				" phone, request_id, order_id, subscription_id ), 32768 );"

			_, err = db.Exec(sql)
			if err != nil {
				errLogger.Println(err)
				continue
			}
		}
	}
}
