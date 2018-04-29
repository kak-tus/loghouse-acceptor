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

var levels = map[int]string{
	0: "FATAL",
	1: "FATAL",
	2: "FATAL",
	3: "ERROR",
	4: "WARN",
	5: "INFO",
	6: "INFO",
	7: "DEBUG",
}

var facilities = map[int]string{
	0:  "kern",
	1:  "user",
	2:  "mail",
	3:  "daemon",
	4:  "auth",
	5:  "syslog",
	6:  "lpr",
	7:  "news",
	8:  "uucp",
	9:  "cron",
	10: "security",
	11: "ftp",
	12: "ntp",
	13: "logaudit",
	14: "logalert",
	15: "clock",
	16: "local0",
	17: "local1",
	18: "local2",
	19: "local3",
	20: "local4",
	21: "local5",
	22: "local6",
	23: "local7",
}

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
	return nil
}
