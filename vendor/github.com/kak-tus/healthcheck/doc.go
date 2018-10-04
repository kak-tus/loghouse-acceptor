/*
Package healthcheck - universal HTTP healthcheck package

Usage example

First, create file with healthcheck configuration section

  // app.yml
	---
	healthcheck:
		listen: ':9000'

Then you can define healthchecks

	package main

	import (
		"git.aqq.me/go/app/appconf"
		"git.aqq.me/go/app/launcher"
		"github.com/iph0/conf/fileconf"
		"github.com/kak-tus/healthcheck"
	)

	func init() {
		fileLdr, err := fileconf.NewLoader("etc")
		if err != nil {
			panic(err)
		}

		appconf.RegisterLoader("file", fileLdr)

		appconf.Require("file:app.yml")
	}

	func main() {
		launcher.Run(func() error {
			healthcheck.Add("/ping", func() (healthcheck.State, string) {
				return healthcheck.StatePassing, "ok"
			})
			healthcheck.Add("/status", func() (healthcheck.State, string) {
				return healthcheck.StateCritical, "err"
			})
			return nil
		})
	}
*/
package healthcheck
