package listener

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

// Listener holds listener object
type Listener struct {
	ResChannel chan Request
	logger     *log.Logger
	errLogger  *log.Logger
}

// New returns new object
func New() *Listener {
	l := &Listener{
		ResChannel: make(chan Request, 1000000),
		logger:     log.New(os.Stdout, "", 0),
		errLogger:  log.New(os.Stderr, "", 0),
	}

	return l
}

// Listen for syslog protocol
func (l *Listener) Listen() error {
	channel := make(syslog.LogPartsChannel, 100000)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.SetTimeout(60000)

	err := server.ListenTCP("0.0.0.0:3333")
	if err != nil {
		return err
	}

	server.Boot()

	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, syscall.SIGINT, syscall.SIGTERM)

	go func(channel syslog.LogPartsChannel) {
		for {
			select {
			case message := <-channel:
				parsed := Request{
					Time:     message["timestamp"].(time.Time),
					Level:    levels[message["severity"].(int)],
					Tag:      message["app_name"].(string),
					Pid:      message["proc_id"].(string),
					Hostname: message["hostname"].(string),
					Facility: facilities[message["facility"].(int)],
					Msg:      strings.Trim(message["message"].(string), " "),
				}

				l.ResChannel <- parsed
			case <-stopSignal:
				l.logger.Println("Got stop signal, stop listening Syslog")
				server.Kill()

				l.logger.Println("Close channel")
				close(l.ResChannel)

				return
			}
		}
	}(channel)

	server.Wait()
	l.logger.Println("Stopped listening Syslog")
	return nil
}
