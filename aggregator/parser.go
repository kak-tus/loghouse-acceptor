package aggregator

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/kak-tus/loghouse-acceptor/listener"
	"github.com/kshvakov/clickhouse"
)

func (a Aggregator) parse(req listener.Request) []interface{} {
	var jsons []interface{}

	str := req.Msg

	// Full json string
	if strings.Index(str, "{") == 0 && str[len(str)-1:] == "}" {
		var parsed interface{}
		err := a.decoder.UnmarshalFromString(str, &parsed)
		if err == nil {
			jsons = append(jsons, parsed)
			str = ""
		}
	} else if strings.Index(str, "{") >= 0 {
		// Special vendor-locked case
		if strings.Index(str, " c{") >= 0 && str[len(str)-1:] == "}" {
			from := strings.Index(str, " c{")

			var parsed interface{}
			err := a.decoder.UnmarshalFromString(str[from+2:], &parsed)
			if err == nil {
				jsons = append(jsons, parsed)
				str = str[:from]
			}
		}

		if strings.Index(str, "{") >= 0 && strings.LastIndex(str, "}") >= 0 {
			from := strings.Index(str, "{")
			to := strings.LastIndex(str, "}")

			var parsed interface{}
			err := a.decoder.UnmarshalFromString(str[from:to+1], &parsed)
			if err == nil {
				jsons = append(jsons, parsed)

				// Special vendor-locked case
				if str[from-1:from] == "j" {
					str = str[:from-1] + str[to+1:]
				} else {
					str = str[:from] + str[to+1:]
				}
			}
		}
	}

	var stringNames []string
	var stringVals []string
	var boolNames []string
	var boolVals []uint8
	var numNames []string
	var numVals []float64
	var nullNames []string

	phone := 0
	requestID := ""
	orderID := ""
	subscriptionID := ""
	caller := ""

	level := req.Level
	tag := req.Tag
	pid := req.Pid

	// Some vendor-locked logic
	// Extract caller, pid, level from string
	if strings.Index(str, "[") == 0 {
		closeBracket := strings.Index(str, "]")
		if closeBracket > 0 {
			firstSpace := strings.Index(str, " ")

			if firstSpace > 0 && firstSpace < closeBracket {
				secondSpace := strings.Index(str[firstSpace+1:], " ")

				if secondSpace > 0 && firstSpace+secondSpace+1 < closeBracket {
					_, err := strconv.Atoi(str[firstSpace+1 : firstSpace+secondSpace+1])

					if err == nil {
						pid = str[firstSpace+1 : firstSpace+secondSpace+1]
						caller = str[firstSpace+secondSpace+2 : closeBracket]
						str = str[closeBracket+1:]

						space := strings.Index(str[1:], " ")
						if space > 0 {
							levelFromStr := str[1 : space+1]

							if levelFromStr == "EMERG" || levelFromStr == "ALERT" ||
								levelFromStr == "CRIT" {
								level = "FATAL"
							} else if levelFromStr == "ERR" {
								level = "ERROR"
							} else if levelFromStr == "WARN" {
								level = "ERROR"
							} else if levelFromStr == "NOTICE" {
								level = "INFO"
							} else if levelFromStr == "INFO" || levelFromStr == "DEBUG" {
								level = levelFromStr
							} else if levelFromStr == "TRACE" {
								level = "DEBUG"
							} else {
								level = "DEBUG"
							}

							str = str[space+2:]
						}
					}
				}
			}
		}
	}

	if len(jsons) > 0 {
		for _, js := range jsons {
			mapped := js.(map[string]interface{})

			for key, val := range mapped {
				// Some vendor-locked logic
				if key == "phone" {
					switch val.(type) {
					case string:
						conv, err := strconv.Atoi(val.(string))
						if err == nil {
							phone = conv
							continue
						}
					case json.Number:
						conv, err := strconv.Atoi(string(val.(json.Number)))
						if err == nil {
							phone = conv
							continue
						}
					}
				} else if key == "request_id" {
					switch val.(type) {
					case string:
						requestID = val.(string)
						continue
					}
				} else if key == "id" {
					_, ok := mapped["request_id"]
					if !ok {
						switch val.(type) {
						case string:
							requestID = val.(string)
							continue
						}
					}
				} else if key == "msg_id" {
					_, ok1 := mapped["request_id"]
					_, ok2 := mapped["id"]
					if !ok1 && !ok2 {
						switch val.(type) {
						case string:
							requestID = val.(string)
							continue
						}
					}
				} else if key == "order_id" {
					switch val.(type) {
					case string:
						orderID = val.(string)
						continue
					}
				} else if key == "subscription_id" {
					switch val.(type) {
					case string:
						subscriptionID = val.(string)
						continue
					}
				} else if key == "level" {
					switch val.(type) {
					case string:
						conv := val.(string)

						if conv == "DEBUG" || conv == "INFO" ||
							conv == "WARN" || conv == "ERROR" ||
							conv == "FATAL" {
							level = conv
						} else if conv == "TRACE" {
							level = "DEBUG"
						} else if conv == "PANIC" {
							level = "FATAL"
						} else {
							level = "DEBUG"
						}

						continue
					}
				} else if key == "tag" {
					switch val.(type) {
					case string:
						tag = val.(string)
						continue
					}
				} else if key == "pid" {
					switch val.(type) {
					case string:
						pid = val.(string)
						continue
					case json.Number:
						pid = string(val.(json.Number))
						continue
					}
				} else if key == "caller" {
					switch val.(type) {
					case string:
						caller = val.(string)
						continue
					}
				}

				switch val.(type) {
				case string:
					stringNames = append(stringNames, key)
					stringVals = append(stringVals, val.(string))
				case bool:
					var conv uint8
					if val.(bool) {
						conv = 1
					} else {
						conv = 0
					}

					boolNames = append(boolNames, key)
					boolVals = append(boolVals, conv)
				case json.Number:
					if strings.Index(string(val.(json.Number)), ".") >= 0 {
						conv, err := strconv.ParseFloat(string(val.(json.Number)), 64)
						if err == nil {
							numNames = append(numNames, key)
							numVals = append(numVals, conv)
						}
					} else {
						conv, err := strconv.Atoi(string(val.(json.Number)))
						if err == nil {
							numNames = append(numNames, key)
							numVals = append(numVals, float64(conv))
						}
					}
				case nil:
					nullNames = append(nullNames, key)
				default:
					conv, err := a.decoder.Marshal(val)
					if err == nil {
						stringNames = append(stringNames, key)
						stringVals = append(stringVals, string(conv))
					}
				}
			}
		}
	}

	res := []interface{}{
		level,
		tag,
		pid,
		caller,
		str,
		clickhouse.Array(stringNames),
		clickhouse.Array(stringVals),
	}

	if len(numNames) > 0 {
		res = append(res, clickhouse.Array(numNames), clickhouse.Array(numVals))
	} else {
		res = append(res, [0]string{}, [0]float64{})
	}

	if len(boolNames) > 0 {
		res = append(res, clickhouse.Array(boolNames), clickhouse.Array(boolVals))
	} else {
		res = append(res, [0]string{}, [0]int{})
	}

	if len(nullNames) > 0 {
		res = append(res, clickhouse.Array(nullNames))
	} else {
		res = append(res, [0]int{})
	}

	res = append(res, phone, requestID, orderID, subscriptionID)

	return res
}
