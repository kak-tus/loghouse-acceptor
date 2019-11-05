package clickhouse

import (
	"database/sql"

	"go.uber.org/zap"
)

// DB handle DB connection object
type DB struct {
	DB           *sql.DB
	logger       *zap.SugaredLogger
	partitionSQL []string
}
