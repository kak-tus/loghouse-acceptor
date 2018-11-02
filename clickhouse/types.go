package clickhouse

import (
	"database/sql"

	"go.uber.org/zap"
)

// DB handle DB connection object
type DB struct {
	DB           *sql.DB
	logger       *zap.SugaredLogger
	partitionSQL string
}

type clickhouseConfig struct {
	Addr             string
	ShardType        string
	PartitionQueries map[string]string
}
