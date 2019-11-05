package clickhouse

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kak-tus/loghouse-acceptor/config"
	"github.com/kshvakov/clickhouse"
	"go.uber.org/zap"
)

// GetDB returns new DB object
func NewDB(cnf config.ClickhouseConfig, log *zap.SugaredLogger) (*DB, error) {
	db, err := sql.Open("clickhouse", "tcp://"+cnf.Addr+"?write_timeout=60")
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		exception, ok := err.(*clickhouse.Exception)
		if ok {
			err = fmt.Errorf("[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace)
		}

		return nil, err
	}

	sql, ok := cnf.PartitionQueries[cnf.ShardType]
	if !ok {
		return nil, errors.New("Unsupported shard type: " + cnf.ShardType)
	}

	dbObj := &DB{
		logger:       log,
		DB:           db,
		partitionSQL: sql,
	}

	log.Info("Started DB")

	return dbObj, nil
}

func (d *DB) Stop() {
	d.logger.Info("Stop DB")
	d.logger.Info("Stopped DB")
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
