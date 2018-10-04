package appconf

import (
	"fmt"

	"git.aqq.me/go/app/event"
	"github.com/iph0/conf"
)

const errPref = "appconf"

var kpr *keeper

func init() {
	kpr = newKeeper()

	initHandler := func() error {
		err := kpr.Init()

		if err != nil {
			return err
		}

		return nil
	}

	event.Init.AddHandler(initHandler)
	event.Reload.AddHandler(initHandler)

	event.Stop.AddHandler(
		func() error {
			kpr.Clean()
			return nil
		},
	)
}

// RegisterLoader method registers configuration loader.
func RegisterLoader(name string, loader conf.Loader) {
	kpr.RegisterLoader(name, loader)
}

// Require method appends configuration locators, which will be used to load
// configuration.
func Require(locators ...interface{}) {
	kpr.Require(locators)
}

// GetConfig returns configuration tree.
func GetConfig() map[string]interface{} {
	config := kpr.GetConfig()

	if config == nil {
		panic(fmt.Errorf("%s must be initialized first", errPref))
	}

	return config
}

// Decode method decodes raw configuration data into structure. Note that the
// conf tags defined in the struct type can indicate which fields the values are
// mapped to.
func Decode(configRaw, config interface{}) error {
	return conf.Decode(configRaw, config)
}
