package clickhouse

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// CreatePartitions start create partitions
func (d *DB) CreatePartitions(partitionFormat string, partitionType string) {
	ticker := time.NewTicker(time.Hour + time.Second*time.Duration(rand.Intn(100)+1))

	for {
		<-ticker.C

		d.logger.Info("Check partitions")

		// Start create partitions from some times ago
		// to allow recreate partitions in case of stopped daemon for some time
		dt := time.Now().AddDate(0, 0, -7)
		to := time.Now().AddDate(0, 0, 7)

		for dt.Before(to) {
			d.create(dt.Format(partitionFormat))

			if partitionType == "hourly" {
				dt = dt.Add(time.Hour)
			} else {
				dt = dt.AddDate(0, 0, 1)
			}
		}
	}
}

func (d *DB) create(partition string) {
	exists, err := d.exists("logs" + partition)
	if err != nil || exists {
		return
	}

	d.logger.Info("Create partition logs" + partition)

	sql := strings.Replace(d.partitionSQL, "__DATE__", partition, -1)

	_, err = d.DB.Exec(sql)
	if err != nil {
		d.logger.Error(err)
	}
}

func (d *DB) exists(table string) (bool, error) {
	sql := "SELECT 1 FROM system.tables WHERE database = 'logs'	AND name = '%s';"

	rows, err := d.DB.Query(fmt.Sprintf(sql, table))
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
