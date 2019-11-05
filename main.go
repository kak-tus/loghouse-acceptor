package main

import (
	"net/http"
	"os"
	"os/signal"

	"github.com/kak-tus/healthcheck"
	"github.com/kak-tus/loghouse-acceptor/config"
	"github.com/kak-tus/loghouse-acceptor/listener"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	log := logger.Sugar()

	cnf, err := config.NewConfig()
	if err != nil {
		log.Panic(err)
	}

	hlth := healthcheck.NewHandler()

	hlth.Add("/healthcheck", func() (healthcheck.State, string) {
		return healthcheck.StatePassing, "ok"
	})

	go func() {
		err := http.ListenAndServe(cnf.Healthcheck.Listen, hlth)
		if err != nil {
			log.Panic(err)
		}
	}()

	lstn, err := listener.NewListener(cnf, log)
	if err != nil {
		log.Panic(err)
	}

	lstn.Listen()

	st := make(chan os.Signal, 1)
	signal.Notify(st, os.Interrupt)

	<-st
	log.Info("Stop")

	err = lstn.Stop()
	if err != nil {
		log.Panic(err)
	}

	_ = log.Sync()
}
