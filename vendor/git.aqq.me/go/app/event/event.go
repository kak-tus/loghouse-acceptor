package event

// Event type represents application event.
type Event struct {
	Handlers []func() error
	Reverse  bool
}

var (
	// Init event instance..
	Init = &Event{
		Handlers: make([]func() error, 0, 10),
	}

	// Reload event instance.
	Reload = &Event{
		Handlers: make([]func() error, 0, 10),
	}

	// Stop event instance.
	Stop = &Event{
		Handlers: make([]func() error, 0, 10),
		Reverse:  true,
	}
)

// AddHandler appends event handler.
func (e *Event) AddHandler(handler func() error) {
	e.Handlers = append(e.Handlers, handler)
}
