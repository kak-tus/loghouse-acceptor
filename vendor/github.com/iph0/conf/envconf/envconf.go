// Copyright (c) 2018, Eugene Ponizovsky, <ponizovsky@gmail.com>. All rights
// reserved. Use of this source code is governed by a MIT License that can
// be found in the LICENSE file.

/*
Package envconf is configuration loader for the conf package. It loads
configuration layers from environment variables. Configuration locators for this
loader are regular expressions. Here some examples:

 env:^MYAPP_
 env:.*
*/
package envconf

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/iph0/conf"
)

const errPref = "envconf"

// EnvLoader loads configuration layers from environment variables.
type EnvLoader struct{}

// NewLoader method creates new loader instance.
func NewLoader() conf.Loader {
	return &EnvLoader{}
}

// Load method loads configuration layer.
func (p *EnvLoader) Load(loc *conf.Locator) (interface{}, error) {
	reStr := loc.BareLocator
	re, err := regexp.Compile(reStr)

	if err != nil {
		return nil, fmt.Errorf("%s: %s", errPref, err)
	}

	envs := os.Environ()
	config := make(map[string]interface{})

	for _, envRaw := range envs {
		pair := strings.SplitN(envRaw, "=", 2)

		if re.MatchString(pair[0]) {
			config[pair[0]] = pair[1]
		}
	}

	return config, nil
}
