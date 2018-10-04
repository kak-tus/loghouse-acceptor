package listener

import (
	"strings"
	"sync"
	"time"

	"git.aqq.me/go/app/applog"
	"git.aqq.me/go/app/event"
	"github.com/kak-tus/loghouse-acceptor/aggregator"
	"github.com/kak-tus/loghouse-acceptor/request"
	syslog "gopkg.in/mcuadros/go-syslog.v2"
)

var listenerObj *Listener

func init() {
	event.Init.AddHandler(
		func() error {
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

			err = server.Boot()
			if err != nil {
				return err
			}

			listenerObj = &Listener{
				logger:     applog.GetLogger().Sugar(),
				server:     server,
				aggregator: aggregator.GetAggregator(),
				channel:    channel,
				m:          &sync.Mutex{},
			}

			listenerObj.logger.Info("Inited listener")

			return nil
		},
	)

	event.Stop.AddHandler(
		func() error {
			listenerObj.logger.Info("Stop listener")

			err := listenerObj.server.Kill()
			if err != nil {
				return err
			}

			close(listenerObj.channel)
			listenerObj.m.Lock()

			listenerObj.aggregator.Stop()

			listenerObj.logger.Info("Stopped listener")

			return nil
		},
	)
}

// GetListener returns object
func GetListener() *Listener {
	return listenerObj
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
