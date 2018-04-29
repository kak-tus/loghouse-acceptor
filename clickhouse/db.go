package clickhouse

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/kshvakov/clickhouse"
)

// DB handle DB connection object
type DB struct {
	DB        *sql.DB
	logger    *log.Logger
	errLogger *log.Logger
}

var db *DB

// New returns new DB object
func New() (*DB, error) {
	d := &DB{
		logger:    log.New(os.Stdout, "", 0),
		errLogger: log.New(os.Stderr, "", 0),
	}

	addr := os.Getenv("CLICKHOUSE_ADDR")

	var err error

	d.DB, err = sql.Open("clickhouse", "tcp://"+addr+"?write_timeout=60")
	if err != nil {
		return nil, err
	}

	err = d.DB.Ping()
	if err != nil {
		exception, ok := err.(*clickhouse.Exception)
		if ok {
			err = fmt.Errorf("[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace)
		}

		return nil, err
	}

	return d, nil
}

// Get returns DB instance
func Get() (*DB, error) {
	if db != nil {
		return db, nil
	}

	var err error

	db, err = New()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Send data to Clickhouse
func (d *DB) Send(reqs map[string][][]interface{}) []error {
	errors := make([]error, 0)

	for sql, v := range reqs {
		tx, err := d.DB.Begin()
		if err != nil {
			d.errLogger.Println(err)
			errors = append(errors, err)
			continue
		}

		stmt, err := tx.Prepare(sql)
		if err != nil {
			d.errLogger.Println(err)
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
			d.errLogger.Println(err)
			errors = append(errors, err)
			continue
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
