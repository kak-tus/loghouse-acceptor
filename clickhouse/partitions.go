package clickhouse

import (
	"fmt"
	"math/rand"
	"time"
)

// CreatePartitions start create partitions
func (d *DB) CreatePartitions(partitionFormat string) {
	ticker := time.NewTicker(time.Hour + time.Second*time.Duration(rand.Intn(100)))

	for {
		<-ticker.C

		d.logger.Info("Check partitions")

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

	d.logger.Info("Create partition logs" + partition)

	prepared := fmt.Sprintf(sqlTable, partition, partition)

	_, err = d.DB.Exec(prepared)
	if err != nil {
		d.logger.Error(err)
	}

	prepared = fmt.Sprintf(sqlShard, partition, partition)

	_, err = d.DB.Exec(prepared)
	if err != nil {
		d.logger.Error(err)
	}
}

func (d *DB) exists(table string) (bool, error) {
	sql := "SELECT 1 FROM system.tables " +
		"WHERE database = 'logs'	AND name = '" +
		table + "';"

	rows, err := d.DB.Query(sql)
	if err != nil {
		d.logger.Error(err)
		return false, err
	}

	if rows.Next() {
		rows.Close()
		return true, nil
	}

	rows.Close()
	return false, nil
}
