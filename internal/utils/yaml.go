package utils

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ReadYAML reads a YAML file and unmarshals it into the provided value.
func ReadYAML(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// ReadYAMLFile reads a YAML file and returns a pointer to the unmarshaled value.
func ReadYAMLFile[T any](path string) (*T, error) {
	var v T
	if err := ReadYAML(path, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// WriteYAML marshals a value to YAML and writes it to a file.
func WriteYAML(path string, v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
