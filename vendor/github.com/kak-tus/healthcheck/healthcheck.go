package healthcheck

import (
	"fmt"
	"net/http"

	"git.aqq.me/go/app/appconf"
	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type healthcheckConfig struct {
	Listen string
}

type server struct {
	logger   *zap.SugaredLogger
	listener *http.Server
}

// State type
type State int

const (
	_ = iota
	// StatePassing - check in passing state
	StatePassing State = iota
	// StateWarning - check in warning state
	StateWarning State = iota
	// StateCritical - check in critical state
	StateCritical State = iota
)

var stateMap = map[State]int{
	1: 200,
	2: 429,
	3: 500,
}

var srv *server

func init() {
	event.Init.AddHandler(
		func() error {
			cnf := appconf.GetConfig()["healthcheck"]

			var config healthcheckConfig
			err := mapstructure.Decode(cnf, &config)
			if err != nil {
				return err
			}

			srv = &server{
				logger: applog.GetLogger().Sugar(),
				listener: &http.Server{
					Addr: config.Listen,
				},
			}

			go func() {
				err = srv.listener.ListenAndServe()
				if err != nil && err != http.ErrServerClosed {
					srv.logger.Error(err)
				}
			}()

			srv.logger.Info("Started healthcheck listener")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			srv.logger.Info("Stop healthcheck listener")

			err := srv.listener.Shutdown(nil)
			if err != nil {
				return err
			}
			return nil
		},
	)
}

// Add add new HTTP healthcheck
func Add(path string, f func() (State, string)) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		srv.logger.Debug("Request ", path)

		state, text := f()

		srv.logger.Debug("Response state: ", state)

		if state != StatePassing {
			w.WriteHeader(stateMap[state])
		}

		_, err := fmt.Fprintf(w, text)
		if err != nil {
			srv.logger.Error(err)
		}
	})
}
