// Copyright (c) 2018, Eugene Ponizovsky, <ponizovsky@gmail.com>. All rights
// reserved. Use of this source code is governed by a MIT License that can
// be found in the LICENSE file.

/*
Package fileconf is configuration loader for the conf package. It loads
configuration layers from YAML, JSON or TOML files. Configuration locators for
this loader are relative pathes or glob patterns. See standart package
path/filepath for more information about syntax of glob patterns. Here some
examples:

 file:myapp/dirs.yml
 file:myapp/servers.toml
 file:myapp/*.json
 file:myapp/*.*
*/
package fileconf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/iph0/conf"
	"github.com/iph0/merger"
	yaml "gopkg.in/yaml.v2"
)

const errPref = "fileconf"

var (
	parsers = map[string]func(bytes []byte) (interface{}, error){
		"yml":  unmarshalYAML,
		"yaml": unmarshalYAML,
		"json": unmarshalJSON,
		"toml": unmarshalTOML,
	}

	fileExtRe = regexp.MustCompile("\\.([^.]+)$")
)

// FileLoader loads configuration layers from YAML, JSON and TOML configuration
// files.
type FileLoader struct {
	dirs []string
}

// NewLoader method creates new loader instance. Method accepts a list of
// directories, in which the loader will search configuration files. The merge
// priority of loaded configuration layers depends on the order of directories.
// Layers loaded from rightmost directory have highest priority.
func NewLoader(dirs ...string) *FileLoader {
	if len(dirs) == 0 {
		panic(fmt.Errorf("%s: no directories specified", errPref))
	}

	return &FileLoader{
		dirs: dirs,
	}
}

// Load method loads configuration layer from YAML, JSON and TOML configuration
// files.
func (l *FileLoader) Load(loc *conf.Locator) (interface{}, error) {
	var config interface{}
	globPattern := loc.BareLocator

	for _, dir := range l.dirs {
		absPattern := filepath.Join(dir, globPattern)
		pathes, err := filepath.Glob(absPattern)

		if err != nil {
			return nil, fmt.Errorf("%s: %s", errPref, err)
		}

		for _, path := range pathes {
			matches := fileExtRe.FindStringSubmatch(path)

			if matches == nil {
				return nil, fmt.Errorf("%s: file extension not specified: %s",
					errPref, path)
			}

			ext := matches[1]
			parser, ok := parsers[ext]

			if !ok {
				return nil, fmt.Errorf("%s: unknown file extension .%s",
					errPref, ext)
			}

			f, err := os.Open(path)

			if err != nil {
				return nil, fmt.Errorf("%s: %s", errPref, err)
			}

			defer f.Close()
			bytes, err := ioutil.ReadAll(f)

			if err != nil {
				return nil, fmt.Errorf("%s: %s", errPref, err)
			}

			data, err := parser(bytes)

			if err != nil {
				return nil, fmt.Errorf("%s: %s", errPref, err)
			}

			config = merger.Merge(config, data)
		}
	}

	return config, nil
}

func unmarshalYAML(bytes []byte) (interface{}, error) {
	var iData interface{}
	err := yaml.Unmarshal(bytes, &iData)

	if err != nil {
		return nil, err
	}

	if iData == nil {
		return nil, nil
	}

	switch data := iData.(type) {
	case map[interface{}]interface{}:
		return adaptYAMLMap(data), nil
	default:
		return data, nil
	}
}

func unmarshalJSON(bytes []byte) (interface{}, error) {
	var iData interface{}
	err := json.Unmarshal(bytes, &iData)

	if err != nil {
		return nil, err
	}

	return iData, nil
}

func unmarshalTOML(bytes []byte) (interface{}, error) {
	var iData interface{}
	err := toml.Unmarshal(bytes, &iData)

	if err != nil {
		return nil, err
	}

	return iData, nil
}

func adaptYAMLMap(from map[interface{}]interface{}) conf.M {
	fromType := reflect.ValueOf(from).Type()
	to := make(conf.M)

	for key, value := range from {
		if value == nil {
			continue
		}

		keyStr := fmt.Sprintf("%v", key)
		valType := reflect.ValueOf(value).Type()

		if fromType == valType {
			to[keyStr] = adaptYAMLMap(value.(map[interface{}]interface{}))
			continue
		}

		to[keyStr] = value
	}

	return to
}
