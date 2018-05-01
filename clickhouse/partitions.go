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
			d.create(dt.Format(partitionFormat))
			dt = dt.Add(time.Hour)
		}
	}
}

func (d *DB) create(partition string) {
	exists, err := d.exists("logs" + partition)
	if err != nil || exists {
		return
	}

	d.logger.Println("Create partition logs" + partition)

	sql := "CREATE TABLE IF NOT EXISTS logs.logs" + partition +
		" ON CLUSTER so" +
		" ( date Date, timestamp DateTime, nsec UInt32, namespace String," +
		" level String, tag String, host String, pid String, caller String," +
		" msg String, labels Nested ( names String, values String )," +
		" string_fields Nested ( names String, values String )," +
		" number_fields Nested ( names String, values Float64 )," +
		" boolean_fields Nested ( names String, values UInt8 )," +
		" `null_fields.names` Array(String), phone UInt64, request_id String," +
		" order_id String, subscription_id String )" +
		" ENGINE = Distributed( 'so', 'logs', 'logs" + partition +
		"_shard', rand() );"

	_, err = d.DB.Exec(sql)
	if err != nil {
		d.errLogger.Println(err)
	}

	sql = "CREATE TABLE IF NOT EXISTS logs.logs" + partition +
		"_shard ON CLUSTER so" +
		" ( date Date, timestamp DateTime, nsec UInt32, namespace String," +
		" level String, tag String, host String, pid String, caller String," +
		" msg String, labels Nested ( names String, values String )," +
		" string_fields Nested ( names String, values String )," +
		" number_fields Nested ( names String, values Float64 )," +
		" boolean_fields Nested ( names String, values UInt8 )," +
		" `null_fields.names` Array(String), phone UInt64, request_id String," +
		" order_id String, subscription_id String )" +
		" ENGINE = ReplicatedMergeTree( " +
		"'/clickhouse/tables/{shard}/logs_logs" + partition +
		"_shard', '{replica}'," +
		" date, ( timestamp, nsec, level, tag, host," +
		" phone, request_id, order_id, subscription_id ), 32768 );"

	_, err = d.DB.Exec(sql)
	if err != nil {
		d.errLogger.Println(err)
	}
}

func (d *DB) exists(table string) (bool, error) {
	sql := "SELECT 1 FROM system.tables " +
		"WHERE database = 'logs'	AND name = '" +
		table + "';"

	rows, err := d.DB.Query(sql)
	if err != nil {
		d.errLogger.Println(err)
		return false, err
	}

	if rows.Next() {
		rows.Close()
		return true, nil
	}

	rows.Close()
	return false, nil
}
