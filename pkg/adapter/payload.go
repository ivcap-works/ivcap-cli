// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Adapted from https://github.com/maxott/magda-cli/blob/main/pkg/adapter/payload.go
package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	log "go.uber.org/zap"
)

type payload struct {
	contentType string
	body        []byte
	headers     *http.Header
	statusCode  int
}

func ToPayload(body []byte, resp *http.Response, logger *log.Logger) Payload {
	contentType := resp.Header.Get("Content-Type")
	logger.Debug("Received", log.String("content-type", contentType))
	return &payload{
		body:        body,
		contentType: contentType,
		headers:     &resp.Header,
		statusCode:  resp.StatusCode,
	}
}

func LoadPayloadFromStdin(isYAML bool) (Payload, error) {
	if data, err := io.ReadAll(os.Stdin); err != nil {
		return nil, err
	} else {
		return LoadPayloadFromBytes(data, isYAML)
	}
}

func LoadPayloadFromFile(fileName string, isYAML bool) (Payload, error) {
	if data, err := os.ReadFile(filepath.Clean(fileName)); err != nil {
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
	var contentType string
	if isYAML {
		contentType = "application/yaml"
	} else {
		contentType = "application/json"
	}
	pyld = &payload{
		body:        data,
		contentType: contentType,
	}
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
	if s, err := ToString(f, useYAML); err == nil {
		fmt.Printf("%s\n", s)
	}
	return
}

func ToString(f any, useYAML bool) (s string, err error) {
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
	return string(b), nil
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

func (p *payload) AsReader() (io.Reader, int64) {
	return bytes.NewReader(p.body), int64(len(p.body))
}

func (p *payload) IsEmpty() bool {
	return len(p.body) == 0
}

func (p *payload) Header(key string) string {
	if p.headers != nil {
		return p.headers.Get(key)
	} else {
		return ""
	}
}

func (p *payload) ContentType() string {
	return p.contentType
}

func (p *payload) StatusCode() int {
	return p.statusCode
}
