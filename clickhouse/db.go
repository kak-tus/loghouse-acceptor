package clickhouse

import (
	"database/sql"
	"fmt"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/iph0/conf"
	"github.com/kshvakov/clickhouse"
)

var dbObj *DB

func init() {
	event.Init.AddHandler(
		func() error {
			cnfMap := appconf.GetConfig()["clickhouse"]

			var cnf clickhouseConfig
			err := conf.Decode(cnfMap, &cnf)
			if err != nil {
				return err
			}

			db, err := sql.Open("clickhouse", "tcp://"+cnf.Addr+"?write_timeout=60")
			if err != nil {
				return err
			}

			err = db.Ping()
			if err != nil {
				exception, ok := err.(*clickhouse.Exception)
				if ok {
					err = fmt.Errorf("[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace)
				}

				return err
			}

			dbObj = &DB{
				logger: applog.GetLogger().Sugar(),
				DB:     db,
			}

			dbObj.logger.Info("Started DB")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			dbObj.logger.Info("Stop DB")
			dbObj.logger.Info("Stopped DB")
			return nil
		},
	)
}

// GetDB returns new DB object
func GetDB() *DB {
	return dbObj
}

// Send data to Clickhouse
func (d *DB) Send(reqs map[string][][]interface{}) []error {
	errors := make([]error, 0)

	for sql, v := range reqs {
		tx, err := d.DB.Begin()
		if err != nil {
			d.logger.Error(err)
			errors = append(errors, err)
			continue
		}

		stmt, err := tx.Prepare(sql)
		if err != nil {
			d.logger.Error(err)
			errors = append(errors, err)
			tx.Rollback()
			continue
		}

		for _, args := range v {
			_, err = stmt.Exec(args...)
			if err != nil {
				tx.Rollback()
				tx = nil
				errors = append(errors, err)
				continue
			}
		}

		if tx == nil {
			continue
		}

		err = tx.Commit()
		if err != nil {
			tx.Rollback()
			d.logger.Error(err)
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
