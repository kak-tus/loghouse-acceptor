package main

import (
	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/launcher"
	"github.com/iph0/conf/envconf"
	"github.com/iph0/conf/fileconf"
	"github.com/kak-tus/healthcheck"
	"github.com/kak-tus/loghouse-acceptor/listener"
)

func init() {
	fileLdr := fileconf.NewLoader("etc", "/etc")
	envLdr := envconf.NewLoader()

	appconf.RegisterLoader("file", fileLdr)
	appconf.RegisterLoader("env", envLdr)

	appconf.Require("file:loghouse-acceptor.yml")
	appconf.Require("env:^ACC_", "env:^CLICKHOUSE_")
}

func main() {
	launcher.Run(func() error {
		healthcheck.Add("/healthcheck", func() (healthcheck.State, string) {
			return healthcheck.StatePassing, "ok"
		})

		syslog := listener.GetListener()
		syslog.Listen()

		return nil
	})
}
