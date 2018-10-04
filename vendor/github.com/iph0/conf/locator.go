package conf

import (
	"fmt"
	"strings"
)

// Locator is used by configuration processor and loaders to load configuration
// layers.
type Locator struct {
	Loader      string
	BareLocator string
}

// ParseLocator method creates Locator instance from the string.
func ParseLocator(rawLoc string) (*Locator, error) {
	if rawLoc == "" {
		return nil, fmt.Errorf("%s: empty configuration locator specified", errPref)
	}

	locTokens := strings.SplitN(rawLoc, ":", 2)

	if len(locTokens) < 2 || locTokens[0] == "" {
		return nil, fmt.Errorf("%s: missing loader name in configuration locator",
			errPref)
	}

	return &Locator{
		Loader:      locTokens[0],
		BareLocator: locTokens[1],
	}, nil
}

// String method convert Locator instance to the string.
func (l *Locator) String() string {
	return fmt.Sprintf("%s:%s", l.Loader, l.BareLocator)
}
