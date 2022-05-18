package adapter

import (
	_ "fmt"
	_ "regexp"
	"testing"

)

func TestYaml(t *testing.T) {
	y := `
foo: 1
`
	p, _ := LoadPayloadFromBytes([]byte(y), true)
	m, _ := p.AsObject()
	switch v := m[`foo`].(type) { 
	default:
		t.Fatalf("unexpected type %T for 'foo'", v)
	case float64:
		if int(v) != 1 {
			t.Fatalf("Expected '1', but is '%f'", v)
		}
	} 
}


func TestYamlArray(t *testing.T) {
	type A struct {
		Value int `json:"a"`
	}
	type T struct {
		Foo []A `json:"foo"`
	}

	y := `
foo:
  - a: 1
  - a: 2
`
	p, err := LoadPayloadFromBytes([]byte(y), true)
	if err != nil {
		t.Fatalf("LoadPayloadFromBytes - %v", err)
	}
	var res T
	if err = p.AsType(&res); err != nil {
		t.Fatalf("Unmarshall - %v", err)
	}

	if len(res.Foo) != 2 {
		t.Fatalf("Expected array of length 2, but got %v", res.Foo)
	}
	for i, item := range res.Foo {
		if item.Value != i + 1 {
			t.Fatalf("Expected item value of %d, but got %d", i + 1, item.Value)
		}
	}
}

func TestYamlNested(t *testing.T) {
	type B struct {
		Value []int `json:"v"`
	}
	type A struct {
		List []B `json:"b"`
	}
	type T struct {
		Foo []A `json:"a"`
	}

	y := `
a: 
  - b: 
    - v: 
      - 1
      - 2
  - b:
`
	p, err := LoadPayloadFromBytes([]byte(y), true)
	if err != nil {
		t.Fatalf("LoadPayloadFromBytes - %v", err)
	}
	var res T
	if err = p.AsType(&res); err != nil {
		t.Fatalf("Unmarshall - %v", err)
	}

	if len(res.Foo) != 2 {
		t.Fatalf("Expected array of length 2, but got %v", res.Foo)
	}
	e1 := res.Foo[0]
	if len(e1.List) != 1 {
		t.Fatalf("Expected array of length 1, but got %v", e1.List)
	}
	for i, item := range e1.List[0].Value {
		if item != i + 1 {
			t.Fatalf("Expected item value of %d, but got %d", i + 1, item)
		}
	}
}
