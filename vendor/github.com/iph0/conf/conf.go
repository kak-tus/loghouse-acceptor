package conf

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/iph0/merger"
	mapstruct "github.com/mitchellh/mapstructure"
)

const (
	genericName    = "conf"
	errPref        = genericName
	decoderTagName = genericName
	varNameSep     = "."
)

var (
	varKey     = reflect.ValueOf("_var")
	includeKey = reflect.ValueOf("_include")
	emptyStr   = reflect.ValueOf("")
	zero       = reflect.ValueOf(nil)
)

// Processor loads configuration layers from different sources and merges them
// into the one configuration tree. In addition configuration processor can
// expand variables in string values and process _var and _include directives in
// resulting configuration tree. Processing can be disabled if not needed.
type Processor struct {
	config      ProcessorConfig
	root        reflect.Value
	breadcrumbs []string
	vars        map[string]reflect.Value
	seen        map[reflect.Value]bool
}

// ProcessorConfig is a structure with configuration parameters for configuration
// processor.
type ProcessorConfig struct {
	// Loaders specifies configuration loaders. Map keys reperesents names of
	// configuration loaders, that further can be used in configuration locators.
	Loaders map[string]Loader

	// DisableProcessing disables expansion of variables and processing of directives.
	DisableProcessing bool
}

// Loader is an interface for configuration loaders.
type Loader interface {
	Load(*Locator) (interface{}, error)
}

// NewProcessor method creates new configuration processor instance.
func NewProcessor(config ProcessorConfig) *Processor {
	if config.Loaders == nil {
		config.Loaders = make(map[string]Loader)
	}

	return &Processor{
		config: config,
	}
}

// Decode method decodes raw configuration data into structure. Note that the
// conf tags defined in the struct type can indicate which fields the values are
// mapped to (see the example below). The decoder will make the following conversions:
//  - bools to string (true = "1", false = "0")
//  - numbers to string (base 10)
//  - bools to int/uint (true = 1, false = 0)
//  - strings to int/uint (base implied by prefix)
//  - int to bool (true if value != 0)
//  - string to bool (accepts: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false,
//    False. Anything else is an error)
//  - empty array = empty map and vice versa
//  - negative numbers to overflowed uint values (base 10)
//  - slice of maps to a merged map
//  - single values are converted to slices if required. Each element also can
//    be converted. For example: "4" can become []int{4} if the target type is
//    an int slice.
func Decode(configRaw, config interface{}) error {
	decoder, err := mapstruct.NewDecoder(
		&mapstruct.DecoderConfig{
			WeaklyTypedInput: true,
			Result:           &config,
			TagName:          decoderTagName,
		},
	)

	if err != nil {
		return err
	}

	err = decoder.Decode(configRaw)

	if err != nil {
		return err
	}

	return nil
}

// Load method loads configuration tree using configuration locators.
// Configuration locator can be a string or a map of type map[string]interface{}.
// Map type can be used to specify default configuration layers. The merge
// priority of loaded configuration layers depends on the order of configuration
// locators. Layers loaded by rightmost locator have highest priority.
func (p *Processor) Load(locators ...interface{}) (map[string]interface{}, error) {
	if len(locators) == 0 {
		panic(fmt.Errorf("%s: no configuration locators specified", errPref))
	}

	iConfig, err := p.load(locators)

	if err != nil {
		return nil, err
	}

	if iConfig == nil {
		return nil, nil
	}

	if !p.config.DisableProcessing {
		iConfig, err = p.process(iConfig)
	}

	if err != nil {
		return nil, err
	}

	config, ok := iConfig.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: loaded configuration has invalid type %T",
			errPref, config)
	}

	return config, nil
}

func (p *Processor) load(locators []interface{}) (interface{}, error) {
	var layer interface{}

	for _, iRawLoc := range locators {
		switch rawLoc := iRawLoc.(type) {
		case map[string]interface{}:
			layer = merger.Merge(layer, rawLoc)
		case string:
			loc, err := ParseLocator(rawLoc)

			if err != nil {
				return nil, err
			}

			loader, ok := p.config.Loaders[loc.Loader]

			if !ok {
				return nil,
					fmt.Errorf("%s: loader not found for configuration locator %s",
						errPref, loc)
			}

			subLayer, err := loader.Load(loc)

			if err != nil {
				return nil, err
			}

			if subLayer == nil {
				continue
			}

			layer = merger.Merge(layer, subLayer)
		default:
			return nil, fmt.Errorf("%s: configuration locator has invalid type %T",
				errPref, rawLoc)
		}
	}

	return layer, nil
}

func (p *Processor) process(config interface{}) (interface{}, error) {
	root := reflect.ValueOf(config)
	p.root = root
	p.breadcrumbs = make([]string, 0, 10)
	p.vars = make(map[string]reflect.Value)
	p.seen = make(map[reflect.Value]bool)

	defer func() {
		p.root = zero
		p.breadcrumbs = nil
		p.vars = nil
		p.seen = nil
	}()

	root, err := p.processNode(root)

	if err != nil {
		return nil, err
	}

	p.root = root
	err = p.walk(root)

	if err != nil {
		return nil, fmt.Errorf("%s at %s", err, p.errContext())
	}

	config = root.Interface()

	return config, nil
}

func (p *Processor) walk(node reflect.Value) error {
	node = reveal(node)
	kind := node.Kind()

	if kind == reflect.Map ||
		kind == reflect.Slice {

		if _, ok := p.seen[node]; ok {
			return nil
		}

		p.seen[node] = true
		var err error

		if kind == reflect.Map {
			err = p.walkMap(node)
		} else {
			err = p.walkSlice(node)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) walkMap(m reflect.Value) error {
	for _, key := range m.MapKeys() {
		iKey := key.Interface()
		p.pushCrumb(iKey.(string))

		node := m.MapIndex(key)
		node, err := p.processNode(node)

		if err != nil {
			return err
		}

		m.SetMapIndex(key, node)
		err = p.walk(node)

		if err != nil {
			return err
		}

		p.popCrumb()
	}

	return nil
}

func (p *Processor) walkSlice(s reflect.Value) error {
	sliceLen := s.Len()

	for i := 0; i < sliceLen; i++ {
		indexStr := strconv.Itoa(i)
		p.pushCrumb(indexStr)

		node := s.Index(i)
		node, err := p.processNode(node)

		if err != nil {
			return err
		}

		s.Index(i).Set(node)
		err = p.walk(node)

		if err != nil {
			return err
		}

		p.popCrumb()
	}

	return nil
}

func (p *Processor) processNode(node reflect.Value) (reflect.Value, error) {
	node = reveal(node)
	kind := node.Kind()
	var err error

	if kind == reflect.String {
		node, err = p.expandVars(node)
	} else if kind == reflect.Map {
		if name := node.MapIndex(varKey); name.IsValid() {
			node, err = p.getVar(name)
		} else if locators := node.MapIndex(includeKey); locators.IsValid() {
			node, err = p.include(locators)
		}
	}

	if err != nil {
		return zero, err
	}

	return node, nil
}

func (p *Processor) expandVars(orig reflect.Value) (reflect.Value, error) {
	var resultStr string
	iOrig := orig.Interface()
	runes := []rune(iOrig.(string))
	runesLen := len(runes)
	i, j := 0, 0

	for j < runesLen {
		if runes[j] == '$' && j+1 < runesLen {
			var esc bool
			k := 1

			if runes[j+1] == '$' {
				esc = true
				k++
			}

			if runes[j+k] == '{' {
				resultStr += string(runes[i:j])

				for i, j = j, j+k+1; j < runesLen; j++ {
					if runes[j] == '}' {
						if esc {
							resultStr += string(runes[i+1 : j+1])
						} else {
							name := string(runes[i+2 : j])

							if len(name) > 0 {
								value, err := p.resolveVar(name)

								if err != nil {
									return zero, err
								}

								resultStr += fmt.Sprintf("%v", value.Interface())
							} else {
								resultStr += string(runes[i : j+1])
							}
						}

						i, j = j+1, j+1

						break
					}
				}

				continue
			}
		}

		j++
	}

	resultStr += string(runes[i:j])
	result := reflect.ValueOf(resultStr)

	return result, nil
}

func (p *Processor) getVar(name reflect.Value) (reflect.Value, error) {
	name = reveal(name)
	kind := name.Kind()

	if kind != reflect.String {
		return zero, fmt.Errorf("%s: invalid _var directive", errPref)
	}

	iName := name.Interface()
	value, err := p.resolveVar(iName.(string))

	if err != nil {
		return zero, err
	}

	return value, nil
}

func (p *Processor) include(locators reflect.Value) (reflect.Value, error) {
	locators = reveal(locators)
	kind := locators.Kind()

	if kind != reflect.Slice {
		return zero, fmt.Errorf("%s: invalid _include directive", errPref)
	}

	iLocators := locators.Interface()
	locsSlice := iLocators.([]interface{})
	layer, err := p.load(locsSlice)

	if err != nil {
		return zero, err
	}

	return reflect.ValueOf(layer), nil
}

func (p *Processor) resolveVar(name string) (reflect.Value, error) {
	if name[0] == '.' {
		nameLen := len(name)
		crumbsLen := len(p.breadcrumbs)
		i := 0

		for ; i < nameLen; i++ {
			if name[i] != '.' {
				break
			}
		}

		if i >= crumbsLen {
			name = name[i:]
		} else {
			baseName := strings.Join(p.breadcrumbs[:crumbsLen-i], varNameSep)

			if i == nameLen {
				name = baseName
			} else {
				name = baseName + varNameSep + name[i:]
			}
		}

		if name == "" {
			return p.root, nil
		}
	}

	value, ok := p.vars[name]

	if ok {
		return value, nil
	}

	value, err := p.findNode(name)

	if err != nil {
		return zero, err
	}

	p.vars[name] = value

	return value, nil
}

func (p *Processor) findNode(name string) (reflect.Value, error) {
	var parent reflect.Value
	node := p.root
	tokens := strings.Split(name, varNameSep)
	tokensLen := len(tokens)

	for i := 0; i < tokensLen; i++ {
		tokens[i] = strings.Trim(tokens[i], " ")
		node = reveal(node)
		kind := node.Kind()

		if kind == reflect.Map {
			parent = node
			key := reflect.ValueOf(tokens[i])

			crumbs := p.breadcrumbs
			p.breadcrumbs = tokens[:i+1]

			var err error
			node = parent.MapIndex(key)
			node, err = p.processNode(node)

			p.breadcrumbs = crumbs

			if err != nil {
				return zero, err
			}

			parent.SetMapIndex(key, node)
		} else if kind == reflect.Slice {
			parent = node
			j, err := strconv.Atoi(tokens[i])

			if err != nil {
				return zero, fmt.Errorf("%s: invalid slice index", errPref)
			} else if j < 0 || j >= parent.Len() {
				return zero, fmt.Errorf("%s: slice index out of range", errPref)
			}

			crumbs := p.breadcrumbs
			p.breadcrumbs = tokens[:i+1]

			node = parent.Index(j)
			node, err = p.processNode(node)

			p.breadcrumbs = crumbs

			if err != nil {
				return zero, err
			}

			parent.Index(j).Set(node)
		} else {
			return emptyStr, nil
		}

		if !node.IsValid() {
			return emptyStr, nil
		}
	}

	return node, nil
}

func (p *Processor) pushCrumb(bc string) {
	p.breadcrumbs = append(p.breadcrumbs, bc)
}

func (p *Processor) popCrumb() {
	p.breadcrumbs = p.breadcrumbs[:len(p.breadcrumbs)-1]
}

func reveal(value reflect.Value) reflect.Value {
	kind := value.Kind()

	if kind == reflect.Interface {
		return value.Elem()
	}

	return value
}

func (p *Processor) errContext() string {
	return strings.Join(p.breadcrumbs, varNameSep)
}
