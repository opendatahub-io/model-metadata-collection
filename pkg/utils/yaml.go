package utils

import (
	"gopkg.in/yaml.v3"
)

// MarshalYAMLWithNewline marshals a value to YAML and ensures the output ends
// with a newline. go-yaml v3's yaml.Marshal uses a default width of 80 but
// does not fold plain scalars (e.g. base64 logos), so long single-line strings
// are preserved as-is.
func MarshalYAMLWithNewline(v any) ([]byte, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Ensure trailing newline
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data, nil
}
