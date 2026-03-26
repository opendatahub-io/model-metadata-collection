package utils

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMarshalYAMLWithNewline(t *testing.T) {
	t.Run("short strings marshal correctly", func(t *testing.T) {
		input := struct {
			Name string `yaml:"name"`
		}{Name: "hello"}

		data, err := MarshalYAMLWithNewline(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(string(data), "name: hello") {
			t.Errorf("expected 'name: hello' in output, got: %s", string(data))
		}
	})

	t.Run("long strings are not wrapped", func(t *testing.T) {
		longValue := strings.Repeat("a", 200)
		input := struct {
			Logo string `yaml:"logo"`
		}{Logo: longValue}

		data, err := MarshalYAMLWithNewline(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		foundLogo := false
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "logo:") {
				foundLogo = true
				if !strings.Contains(line, longValue) {
					t.Errorf("long string was wrapped; logo line: %s", line)
				}
				break
			}
		}
		if !foundLogo {
			t.Fatal("expected 'logo:' field in YAML output, but it was not found")
		}
	})

	t.Run("output ends with newline", func(t *testing.T) {
		input := struct {
			Name string `yaml:"name"`
		}{Name: "test"}

		data, err := MarshalYAMLWithNewline(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			t.Error("output does not end with newline")
		}
	})

	t.Run("round trip preserves structure", func(t *testing.T) {
		type testStruct struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
			Count int    `yaml:"count"`
		}
		original := testStruct{Name: "test", Value: "hello world", Count: 42}

		data, err := MarshalYAMLWithNewline(original)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}

		var roundTripped testStruct
		if err := yaml.Unmarshal(data, &roundTripped); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if original != roundTripped {
			t.Errorf("round-trip mismatch: original=%+v, got=%+v", original, roundTripped)
		}
	})

	t.Run("base64 logo string survives round trip", func(t *testing.T) {
		logo := "data:image/svg+xml;base64," + strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", 60)

		type testStruct struct {
			Name string `yaml:"name"`
			Logo string `yaml:"logo"`
		}
		original := testStruct{Name: "test-server", Logo: logo}

		data, err := MarshalYAMLWithNewline(original)
		if err != nil {
			t.Fatalf("unexpected marshal error: %v", err)
		}

		var roundTripped testStruct
		if err := yaml.Unmarshal(data, &roundTripped); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}

		if original.Logo != roundTripped.Logo {
			t.Errorf("logo was corrupted during round-trip: original length=%d, got length=%d", len(original.Logo), len(roundTripped.Logo))
		}
	})
}
