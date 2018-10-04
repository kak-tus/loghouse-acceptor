package app

import "git.aqq.me/go/app/event"

// Init method initialize application components.
func Init() error {
	return sendEvent(event.Init)
}

// Reload method performs data reloading.
func Reload() error {
	return sendEvent(event.Reload)
}

// Stop method correctly stops application.
func Stop() error {
	return sendEvent(event.Stop)
}

func sendEvent(e *event.Event) error {
	if e.Reverse {
		for i := len(e.Handlers) - 1; i >= 0; i-- {
			err := e.Handlers[i]()

			if err != nil {
				return err
			}
		}
	} else {
		for _, handler := range e.Handlers {
			err := handler()

			if err != nil {
				return err
			}
		}
	}

	return nil
}
