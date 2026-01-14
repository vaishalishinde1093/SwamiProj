package main

import "gopkg.in/yaml.v3"

func init() {
	marshalYAML = func(v any) ([]byte, error) {
		return yaml.Marshal(v)
	}
}
