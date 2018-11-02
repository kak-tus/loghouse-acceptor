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
	refNameSep     = "."
)

var (
	refKey      = reflect.ValueOf("_ref")
	nameKey     = reflect.ValueOf("name")
	firstDefKey = reflect.ValueOf("firstDefined")
	defaultKey  = reflect.ValueOf("default")

	includeKey = reflect.ValueOf("_include")
)

// Processor loads configuration layers from different sources and merges them
// into the one configuration tree. In addition configuration processor can
// expand references on configuration parameters in string values and process
// _ref and _include directives in resulting configuration tree. Processing can
// be disabled if not needed.
type Processor struct {
	config      ProcessorConfig
	root        reflect.Value
	breadcrumbs []string
	refs        map[string]reflect.Value
	seen        map[reflect.Value]struct{}
}

// ProcessorConfig is a structure with configuration parameters for configuration
// processor.
type ProcessorConfig struct {
	// Loaders specifies configuration loaders. Map keys reperesents names of
	// configuration loaders, that further can be used in configuration locators.
	Loaders map[string]Loader

	// DisableProcessing disables expansion of references and processing of
	// directives.
	DisableProcessing bool
}

// Loader is an interface for configuration loaders.
type Loader interface {
	Load(*Locator) (interface{}, error)
}

// M type is a convenient alias for a map[string]interface{} map.
type M = map[string]interface{}

// S type is a convenient alias for a []interface{} slice.
type S = []interface{}

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
			Result:           config,
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
func (p *Processor) Load(locators ...interface{}) (M, error) {
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

	config, ok := iConfig.(M)
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
		case M:
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
			return nil,
				fmt.Errorf("%s: configuration locator must be of type"+
					" map[string]interface{} or string, but has type %T", errPref,
					rawLoc)
		}
	}

	return layer, nil
}

func (p *Processor) process(config interface{}) (interface{}, error) {
	configRf := reflect.ValueOf(config)

	p.root = configRf
	p.breadcrumbs = make([]string, 0, 10)
	p.refs = make(map[string]reflect.Value)
	p.seen = make(map[reflect.Value]struct{})

	defer func() {
		p.root = reflect.Value{}
		p.breadcrumbs = nil
		p.refs = nil
		p.seen = nil
	}()

	configRf, err := p.processNode(configRf)

	if err != nil {
		return nil, err
	}

	p.root = configRf
	err = p.walk(configRf)

	if err != nil {
		return nil, fmt.Errorf("%s at %s", err, p.errContext())
	}

	return configRf.Interface(), nil
}

func (p *Processor) walk(node reflect.Value) error {
	node = reveal(node)
	nodeKind := node.Kind()

	if nodeKind != reflect.Map &&
		nodeKind != reflect.Slice {

		return nil
	}

	if _, ok := p.seen[node]; ok {
		return nil
	}

	p.seen[node] = struct{}{}

	if nodeKind == reflect.Map {
		err := p.walkMap(node)

		if err != nil {
			return err
		}
	} else {
		err := p.walkSlice(node)

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Processor) walkMap(m reflect.Value) error {
	for _, key := range m.MapKeys() {
		keyStr := key.Interface().(string)
		p.pushCrumb(keyStr)

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

	switch node.Kind() {
	case reflect.String:
		str := node.Interface().(string)
		str, err := p.expandRefs(str)

		if err != nil {
			return reflect.Value{}, err
		}

		return reflect.ValueOf(str), nil
	case reflect.Map:
		if data := node.MapIndex(refKey); data.IsValid() {
			node, err := p.processRef(data)

			if err != nil {
				return reflect.Value{}, err
			}

			return node, nil
		} else if locators := node.MapIndex(includeKey); locators.IsValid() {
			node, err := p.processInc(locators)

			if err != nil {
				return reflect.Value{}, err
			}

			return node, nil
		}
	}

	return node, nil
}

func (p *Processor) expandRefs(str string) (string, error) {
	var res string
	runes := []rune(str)
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
				res += string(runes[i:j])

				for i, j = j, j+k+1; j < runesLen; j++ {
					if runes[j] == '}' {
						if esc {
							res += string(runes[i+1 : j+1])
						} else {
							name := string(runes[i+2 : j])

							if len(name) > 0 {
								value, err := p.resolveRef(name)

								if err != nil {
									return "", err
								}

								if value.IsValid() {
									res += fmt.Sprintf("%v", value.Interface())
								}
							} else {
								res += string(runes[i : j+1])
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

	res += string(runes[i:j])

	return res, nil
}

func (p *Processor) processRef(data reflect.Value) (reflect.Value, error) {
	data = reveal(data)

	switch data.Kind() {
	case reflect.String:
		nameStr := data.Interface().(string)
		node, err := p.resolveRef(nameStr)

		if err != nil {
			return reflect.Value{}, err
		}

		return node, nil
	case reflect.Map:
		if name := data.MapIndex(nameKey); name.IsValid() {
			name = reveal(name)
			nameKind := name.Kind()

			if nameKind != reflect.String {
				return reflect.Value{},
					fmt.Errorf("%s: reference name must be of type string, but has "+
						"type %s", errPref, nameKind)
			}

			nameStr := name.Interface().(string)
			node, err := p.resolveRef(nameStr)

			if err != nil {
				return reflect.Value{}, err
			}

			if node.IsValid() {
				return node, nil
			}
		} else if names := data.MapIndex(firstDefKey); names.IsValid() {
			names = reveal(names)
			namesKind := names.Kind()

			if namesKind != reflect.Slice {
				return reflect.Value{},
					fmt.Errorf("%s: firstDefined list must be of type slice, but has "+
						"type %s", errPref, namesKind)
			}

			namesLen := names.Len()

			for i := 0; i < namesLen; i++ {
				name := names.Index(i)
				name = reveal(name)

				if name.Kind() != reflect.String {
					return reflect.Value{},
						fmt.Errorf("%s: reference name in firstDefined list must be of "+
							"type string, but has type %s", errPref, name.Type())
				}

				nameStr := name.Interface().(string)
				node, err := p.resolveRef(nameStr)

				if err != nil {
					return reflect.Value{}, err
				}

				if node.IsValid() {
					return node, nil
				}
			}
		}

		if node := data.MapIndex(defaultKey); node.IsValid() {
			return node, nil
		}
	default:
		return reflect.Value{},
			fmt.Errorf("%s: invalid _ref directive", errPref)
	}

	return reflect.Value{}, nil
}

func (p *Processor) processInc(locators reflect.Value) (reflect.Value, error) {
	locators = reveal(locators)

	if locators.Kind() != reflect.Slice {
		return reflect.Value{},
			fmt.Errorf("%s: invalid _include directive", errPref)
	}

	locsSlice := locators.Interface().([]interface{})
	layer, err := p.load(locsSlice)

	if err != nil {
		return reflect.Value{}, err
	}

	return reflect.ValueOf(layer), nil
}

func (p *Processor) resolveRef(name string) (reflect.Value, error) {
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
			baseName := strings.Join(p.breadcrumbs[:crumbsLen-i], refNameSep)

			if i == nameLen {
				name = baseName
			} else {
				name = baseName + refNameSep + name[i:]
			}
		}

		if name == "" {
			return p.root, nil
		}
	}

	value, ok := p.refs[name]

	if ok {
		return value, nil
	}

	value, err := p.findNode(name)

	if err != nil {
		return reflect.Value{}, err
	}

	p.refs[name] = value

	return value, nil
}

func (p *Processor) findNode(name string) (reflect.Value, error) {
	var parent reflect.Value
	node := p.root
	tokens := strings.Split(name, refNameSep)
	tokensLen := len(tokens)

	for i := 0; i < tokensLen; i++ {
		tokens[i] = strings.Trim(tokens[i], " ")
		node = reveal(node)
		nodeKind := node.Kind()

		if nodeKind == reflect.Map {
			parent = node
			key := reflect.ValueOf(tokens[i])

			crumbs := p.breadcrumbs
			p.breadcrumbs = tokens[:i+1]

			var err error
			node = parent.MapIndex(key)
			node, err = p.processNode(node)

			if err != nil {
				return reflect.Value{}, err
			}

			p.breadcrumbs = crumbs
			parent.SetMapIndex(key, node)
		} else if nodeKind == reflect.Slice {
			parent = node
			j, err := strconv.Atoi(tokens[i])

			if err != nil {
				return reflect.Value{}, fmt.Errorf("%s: invalid slice index", errPref)
			} else if j < 0 || j >= parent.Len() {
				return reflect.Value{},
					fmt.Errorf("%s: slice index out of range", errPref)
			}

			crumbs := p.breadcrumbs
			p.breadcrumbs = tokens[:i+1]

			node = parent.Index(j)
			node, err = p.processNode(node)

			if err != nil {
				return reflect.Value{}, err
			}

			p.breadcrumbs = crumbs
			parent.Index(j).Set(node)
		} else {
			return reflect.Value{}, nil
		}

		if !node.IsValid() {
			return reflect.Value{}, nil
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
	if value.Kind() == reflect.Interface {
		return value.Elem()
	}

	return value
}

func (p *Processor) errContext() string {
	return strings.Join(p.breadcrumbs, refNameSep)
}
