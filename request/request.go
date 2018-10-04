package request

import "time"

// Request struct
type Request struct {
	Time     time.Time
	Level    string
	Tag      string
	Pid      string
	Hostname string
	Facility string
	Msg      string
}
