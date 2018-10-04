package applog

import (
	"fmt"

	"git.aqq.me/go/app/event"
	"go.uber.org/zap"
)

const errPref = "applog"

var logger *Logger

func init() {
	initHandler := func() error {
		var err error
		logger, err = NewLogger()

		if err != nil {
			return err
		}

		return nil
	}

	event.Init.AddHandler(initHandler)
	event.Reload.AddHandler(initHandler)

	event.Stop.AddHandler(
		func() error {
			if logger != nil {
				logger.Close()
				logger = nil
			}

			return nil
		},
	)
}

// GetLogger returns zap logger.
func GetLogger() *zap.Logger {
	if logger == nil {
		panic(fmt.Errorf("%s must be initialized first", errPref))
	}

	return logger.Logger
}
