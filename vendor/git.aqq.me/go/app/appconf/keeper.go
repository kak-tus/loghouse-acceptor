package appconf

import (
	"fmt"

	"github.com/iph0/conf"
)

type keeper struct {
	loaders  map[string]conf.Loader
	locators []interface{}
	locIndex map[string]bool
	config   map[string]interface{}
}

func newKeeper() *keeper {
	return &keeper{
		loaders:  make(map[string]conf.Loader),
		locators: make([]interface{}, 0, 5),
		locIndex: make(map[string]bool),
	}
}

func (k *keeper) RegisterLoader(name string, loader conf.Loader) {
	k.loaders[name] = loader
}

func (k *keeper) Require(locators []interface{}) {
	for _, iLoc := range locators {
		switch loc := iLoc.(type) {
		case map[string]interface{}:
			k.locators = append(k.locators, loc)
		case string:
			if k.locIndex[loc] {
				return
			}

			k.locIndex[loc] = true
			k.locators = append(k.locators, loc)
		default:
			panic(fmt.Sprintf("%s: configuration locator has invalid type %T",
				errPref, loc))
		}
	}
}

func (k *keeper) Init() error {
	configProc := conf.NewProcessor(
		conf.ProcessorConfig{
			Loaders: k.loaders,
		},
	)

	config, err := configProc.Load(k.locators...)

	if err != nil {
		return err
	}

	if config != nil {
		k.config = config
	} else {
		k.config = make(map[string]interface{})
	}

	return nil
}

func (k *keeper) GetConfig() map[string]interface{} {
	return k.config
}

func (k *keeper) Clean() {
	k.config = nil
}
