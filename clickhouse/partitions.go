package clickhouse

import (
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// CreatePartitions start create partitions
func (d *DB) CreatePartitions(partitionFormat string) {
	ticker := time.NewTicker(time.Hour + time.Second*time.Duration(rand.Intn(100)))

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
		case <-stopSignal:
			d.logger.Println("Got stop signal, stop listening create partitions")
			ticker.Stop()
			return
		}

		d.logger.Println("Check partitions")

		// Start create partitions from some times ago
		// to allow recreate partitions in case of stopped daemon for some time
		dt := time.Now().Add(-time.Hour * 24)

		for i := 1; i <= 48; i++ {
			partition := dt.Format(partitionFormat)
			dt = dt.Add(time.Hour)

			sql := "SELECT 1 FROM system.tables WHERE database = 'logs'	AND name = 'logs" +
				partition + "';"

			rows, err := d.DB.Query(sql)
			if err != nil {
				d.errLogger.Println(err)
				continue
			}

			if rows.Next() {
				rows.Close()
				continue
			}

			rows.Close()
			d.logger.Println("Create partition " + partition)

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

			_, err = d.DB.Exec(sql)
			if err != nil {
				d.errLogger.Println(err)
				continue
			}
		}
	}
}
