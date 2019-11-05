package config

import (
	"github.com/iph0/conf"
	"github.com/iph0/conf/envconf"
	"github.com/iph0/conf/fileconf"
)

type Config struct {
	Aggregator  AggregatorConfig
	Clickhouse  ClickhouseConfig
	Healthcheck healthcheckConfig
}

type AggregatorConfig struct {
	PartitionType   string
	PartitionTypes  map[string]string
	Period          int
	Batch           int
	InsertQueryType string
	InsertQueries   map[string]string
}

type ClickhouseConfig struct {
	Addr             string
	ShardType        string
	PartitionQueries map[string][]string
}

type healthcheckConfig struct {
	Listen string
}

func NewConfig() (*Config, error) {
	fileLdr := fileconf.NewLoader("etc", "/etc")
	envLdr := envconf.NewLoader()

	configProc := conf.NewProcessor(
		conf.ProcessorConfig{
			Loaders: map[string]conf.Loader{
				"file": fileLdr,
				"env":  envLdr,
			},
		},
	)

	configRaw, err := configProc.Load(
		"file:loghouse-acceptor.yml",
		"env:^ACC_",
		"env:^CLICKHOUSE_",
	)

	if err != nil {
		return nil, err
	}

	var cnf Config
	if err := conf.Decode(configRaw, &cnf); err != nil {
		return nil, err
	}

	return &cnf, nil
}
