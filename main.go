package main

import (
	"sync"

	"github.com/kak-tus/loghouse-acceptor/aggregator"
	"github.com/kak-tus/loghouse-acceptor/healthcheck"
	"github.com/kak-tus/loghouse-acceptor/listener"
)

func main() {
	syslog := listener.New()

	agg, err := aggregator.New(syslog.ResChannel)
	if err != nil {
		panic(err)
	}

	check, err := healthcheck.New()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	wg.Add(3)

	go func() {
		err := syslog.Listen()
		if err != nil {
			panic(err)
		}

		wg.Done()
	}()

	go func() {
		agg.Aggregate()
		wg.Done()
	}()

	go func() {
		err := check.Listen()
		if err != nil {
			panic(err)
		}

		wg.Done()
	}()

	wg.Wait()
	println("Exit")
}
