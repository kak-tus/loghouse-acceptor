package healthcheck

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kak-tus/loghouse-acceptor/clickhouse"
)

// Healthcheck holds healthcheck object
type Healthcheck struct {
	db        *clickhouse.DB
	logger    *log.Logger
	errLogger *log.Logger
	srv       *http.Server
}

// New returns new healthcheck object
func New() (*Healthcheck, error) {
	db, err := clickhouse.Get()
	if err != nil {
		return nil, err
	}

	h := &Healthcheck{
		db:        db,
		logger:    log.New(os.Stdout, "", 0),
		errLogger: log.New(os.Stderr, "", 0),
		srv:       &http.Server{Addr: ":9001"},
	}
	return h, nil
}

// Listen healthchecks
func (h *Healthcheck) Listen() error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 10; i++ {
			err := h.db.DB.Ping()
			if err == nil {
				fmt.Fprintf(w, "ok")
				return
			}

			h.errLogger.Println(err)
			time.Sleep(time.Second)
		}

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "err")
	})

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-stopSignal
		h.logger.Println("Got stop signal, stop listening HTTP")

		err := h.srv.Shutdown(nil)
		if err != nil {
			h.errLogger.Println(err)
		}
	}()

	err := h.srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
