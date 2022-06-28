// Adapted from https://github.com/maxott/magda-cli/blob/main/pkg/adapter/payload.go
package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"

	log "go.uber.org/zap"
)

type payload struct {
	contentType string
	body        []byte
}

func ToPayload(body []byte, contentType string, logger *log.Logger) (Payload, error) {
	logger.Debug("Received", log.String("content-type", contentType))
	return &payload{body: body, contentType: contentType}, nil
}

func LoadPayloadFromStdin(isYAML bool) (Payload, error) {
	if data, err := ioutil.ReadAll(os.Stdin); err != nil {
		return nil, err
	} else {
		return LoadPayloadFromBytes(data, isYAML)
	}
}

func LoadPayloadFromFile(fileName string, isYAML bool) (Payload, error) {
	if data, err := ioutil.ReadFile(fileName); err != nil {
		return nil, err
	} else {
		return LoadPayloadFromBytes(data, isYAML)
	}
}

func LoadPayloadFromBytes(data []byte, isYAML bool) (pyld Payload, err error) {
	if isYAML {
		obj := make(map[interface{}]interface{})
		if err = yaml.Unmarshal(data, &obj); err != nil {
			return
		}
		if data, err = yamlToJSON(obj); err != nil {
			return
		}
	}
	pyld = &payload{body: data}
	return
}

func yamlToJSON(yamlData map[interface{}]interface{}) ([]byte, error) {
	cleanedYaml, err := cleanYaml(yamlData)
	if err != nil {
		return nil, err
	}
	output, err := json.Marshal(cleanedYaml)
	if err != nil {
		return nil, fmt.Errorf("error converting yaml to json: %s", err.Error())
	}
	return output, nil
}

// fixed version from the one found in "github.com/jdockerty/yaml-to-json-go/conversion"
func cleanYaml(in map[interface{}]interface{}) (output map[string]interface{}, err error) {
	output = make(map[string]interface{})
	for key, value := range in {
		skey := key.(string) // expected to be 'string'
		output[skey] = value

		mval, isMap := value.(map[interface{}]interface{})
		sval, isSlice := value.([]interface{})

		if isMap {
			if output[skey], err = cleanYaml(mval); err != nil {
				return
			}
		} else if isSlice {
			if output[skey], err = cleanArrayYaml(sval); err != nil {
				return
			}
		}
	}
	return
}

func cleanArrayYaml(in []interface{}) (output []interface{}, err error) {
	output = make([]interface{}, len(in))
	for i, item := range in {
		if mslice, isSlice := item.([]interface{}); isSlice {
			if output[i], err = cleanArrayYaml(mslice); err != nil {
				return
			}
		} else if mitem, isMap := item.(map[interface{}]interface{}); isMap {
			if output[i], err = cleanYaml(mitem); err != nil {
				return
			}
		} else {
			output[i] = item
		}
	}
	return
}

func ReplyPrinter(pld Payload, useYAML bool) (err error) {
	var f interface{}
	if err = pld.AsType(&f); err != nil {
		return
	}
	var b []byte
	if useYAML {
		if b, err = yaml.Marshal(f); err != nil {
			return
		}
	} else {
		if b, err = json.MarshalIndent(f, "", "  "); err != nil {
			return
		}
	}
	fmt.Printf("%s\n", b)
	return
}

func (p *payload) AsType(r interface{}) error {
	return json.Unmarshal(p.body, r)
}

func (p *payload) AsObject() (map[string]interface{}, error) {
	var f interface{}
	err := json.Unmarshal(p.body, &f)
	if err != nil {
		return nil, err
	}
	if obj, ok := f.(map[string]interface{}); ok {
		return obj, nil
	} else {
		return nil, errors.New("not an object type")
	}
}

func (p *payload) AsArray() ([]interface{}, error) {
	var f interface{}
	err := json.Unmarshal(p.body, &f)
	if err != nil {
		return nil, err
	}
	switch m := f.(type) {
	case []interface{}:
		return m, nil
	case map[string]interface{}:
		return []interface{}{m}, nil
	default:
		return nil, errors.New("not an array type")
	}
}

func (p *payload) AsBytes() []byte {
	return p.body
}

// type JsonObjPayload struct {
// 	payload map[string]interface{}
// 	bytes   []byte
// }

// func (p JsonObjPayload) IsObject() bool                   { return true }
// func (p JsonObjPayload) AsObject() map[string]interface{} { return p.payload }
// func (p JsonObjPayload) AsArray() []interface{}           { return []interface{}{p.payload} }
// func (p JsonObjPayload) AsBytes() []byte                  { return p.bytes }

// type JsonArrPayload struct {
// 	payload []interface{}
// 	bytes   []byte
// }

// func (JsonArrPayload) IsObject() bool                     { return false }
// func (p JsonArrPayload) AsObject() map[string]interface{} { return nil }
// func (p JsonArrPayload) AsArray() []interface{}           { return p.payload }
// func (p JsonArrPayload) AsBytes() []byte                  { return p.bytes }