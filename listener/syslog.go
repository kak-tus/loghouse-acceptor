package listener

import (
	"strings"
	"sync"
	"time"

	"github.com/kak-tus/loghouse-acceptor/aggregator"
	"github.com/kak-tus/loghouse-acceptor/config"
	"github.com/kak-tus/loghouse-acceptor/request"
	"go.uber.org/zap"
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

func NewListener(cnf *config.Config, log *zap.SugaredLogger) (*Listener, error) {
	channel := make(syslog.LogPartsChannel, 100000)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.SetTimeout(60000)

	err := server.ListenTCP("0.0.0.0:3333")
	if err != nil {
		return nil, err
	}

	err = server.Boot()
	if err != nil {
		return nil, err
	}

	agg, err := aggregator.NewAggregator(cnf, log)
	if err != nil {
		return nil, err
	}

	listenerObj := &Listener{
		logger:     log,
		server:     server,
		aggregator: agg,
		channel:    channel,
		m:          &sync.Mutex{},
	}

	listenerObj.logger.Info("Inited listener")

	return listenerObj, nil
}

func (l *Listener) Stop() error {
	l.logger.Info("Stop listener")

	err := l.server.Kill()
	if err != nil {
		return err
	}

	close(l.channel)
	l.m.Lock()

	l.aggregator.Stop()

	l.logger.Info("Stopped listener")

	return nil
}

// Listen for syslog protocol
func (l *Listener) Listen() {
	l.aggregator.Start()

	go func() {
		l.m.Lock()

		for {
			message, more := <-l.channel

			if !more {
				break
			}

			parsed := request.Request{
				Time:     message["timestamp"].(time.Time),
				Level:    levels[message["severity"].(int)],
				Tag:      message["app_name"].(string),
				Pid:      message["proc_id"].(string),
				Hostname: message["hostname"].(string),
				Facility: facilities[message["facility"].(int)],
				Msg:      strings.Trim(message["message"].(string), " "),
			}

			l.aggregator.C <- parsed
		}

		l.m.Unlock()
	}()

	go l.server.Wait()
}
