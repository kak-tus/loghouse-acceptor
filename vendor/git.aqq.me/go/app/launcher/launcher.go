package launcher

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"git.aqq.me/go/app"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"go.uber.org/zap"
)

var lchr *launcher

type launcher struct {
	logger  *zap.Logger
	reload  chan struct{}
	stop    chan struct{}
	stopped bool
}

func init() {
	lchr = &launcher{
		reload: make(chan struct{}, 1),
		stop:   make(chan struct{}),
	}

	event.Init.AddHandler(
		func() error {
			lchr.logger = applog.GetLogger()
			return nil
		},
	)
}

// Run method launches an application
func Run(appStart func() error) {
	lchr.Run(appStart)
}

// Stop method stops an application
func Stop() {
	lchr.Stop()
}

func (l *launcher) Run(appStart func() error) {
	err := app.Init()

	if err != nil {
		fmt.Fprintln(os.Stderr, "Start failed:", err)
		return
	}

	l.listenSignals()
	err = appStart()

	if err != nil {
		l.logger.Error("Start failed: " + err.Error())
		return
	}

	l.logger.Info("Started")

LOOP:
	for {
		select {
		case <-l.stop:
			break LOOP
		case <-l.reload:
			err := app.Reload()

			if err != nil {
				l.logger.Error("Reload failed: " + err.Error())
				continue LOOP
			}

			l.logger.Info("Reloaded")
		}
	}

	err = app.Stop()

	if err != nil {
		l.logger.Error("Stopped with error: " + err.Error())
		return
	}
}

func (l *launcher) Stop() {
	if l.stopped {
		return
	}

	close(l.stop)
	l.stopped = true
}

func (l *launcher) listenSignals() {
	signals := make(chan os.Signal, 1)

	signal.Notify(signals,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	go func() {
		for sig := range signals {
			l.logger.Info("Got signal: " + sig.String())

			if sig == syscall.SIGHUP {
				l.reload <- struct{}{}
			} else {
				l.Stop()
			}
		}
	}()
}
