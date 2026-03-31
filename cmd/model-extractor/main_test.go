package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		presetEnv   map[string]string // env vars set before loading
		expected    map[string]string // expected env vars after loading
		notExpected []string          // env var keys that must NOT be set
	}{
		{
			name:    "basic key=value",
			content: "MY_KEY=my_value\n",
			expected: map[string]string{
				"MY_KEY": "my_value",
			},
		},
		{
			name:    "double-quoted value",
			content: `MY_KEY="quoted value"` + "\n",
			expected: map[string]string{
				"MY_KEY": "quoted value",
			},
		},
		{
			name:    "single-quoted value",
			content: `MY_KEY='single quoted'` + "\n",
			expected: map[string]string{
				"MY_KEY": "single quoted",
			},
		},
		{
			name:    "comments and blank lines skipped",
			content: "# comment\n\nKEY1=val1\n  # indented comment\nKEY2=val2\n",
			expected: map[string]string{
				"KEY1": "val1",
				"KEY2": "val2",
			},
		},
		{
			name:    "shell export takes precedence",
			content: "MY_KEY=from_env_file\n",
			presetEnv: map[string]string{
				"MY_KEY": "from_shell",
			},
			expected: map[string]string{
				"MY_KEY": "from_shell",
			},
		},
		{
			name:    "empty value",
			content: "MY_KEY=\n",
			expected: map[string]string{
				"MY_KEY": "",
			},
		},
		{
			name:    "value with equals sign",
			content: "MY_KEY=a=b=c\n",
			expected: map[string]string{
				"MY_KEY": "a=b=c",
			},
		},
		{
			name:        "line without equals is skipped",
			content:     "NO_EQUALS\nGOOD_KEY=good\n",
			notExpected: []string{"NO_EQUALS"},
			expected: map[string]string{
				"GOOD_KEY": "good",
			},
		},
		{
			name:    "whitespace around key and value is trimmed",
			content: "  MY_KEY  =  my_value  \n",
			expected: map[string]string{
				"MY_KEY": "my_value",
			},
		},
		{
			name:    "export prefix is stripped",
			content: "export MY_KEY=exported_value\n",
			expected: map[string]string{
				"MY_KEY": "exported_value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Collect all keys to clean up
			var allKeys []string
			for k := range tt.expected {
				allKeys = append(allKeys, k)
			}
			for k := range tt.presetEnv {
				allKeys = append(allKeys, k)
			}
			allKeys = append(allKeys, tt.notExpected...)

			// Clear and restore env vars (use LookupEnv to distinguish "not set" from "set to empty")
			type envEntry struct {
				val string
				set bool
			}
			origValues := make(map[string]envEntry)
			for _, k := range allKeys {
				v, ok := os.LookupEnv(k)
				origValues[k] = envEntry{v, ok}
				_ = os.Unsetenv(k)
			}
			defer func() {
				for k, e := range origValues {
					if e.set {
						_ = os.Setenv(k, e.val)
					} else {
						_ = os.Unsetenv(k)
					}
				}
			}()

			// Set pre-existing env vars
			for k, v := range tt.presetEnv {
				_ = os.Setenv(k, v)
			}

			// Write .env file
			tmpDir := t.TempDir()
			envPath := filepath.Join(tmpDir, ".env")
			if err := os.WriteFile(envPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test .env: %v", err)
			}

			loadDotEnv(envPath)

			for k, want := range tt.expected {
				got := os.Getenv(k)
				if got != want {
					t.Errorf("env %s = %q, want %q", k, got, want)
				}
			}
			for _, k := range tt.notExpected {
				if got, ok := os.LookupEnv(k); ok {
					t.Errorf("env %s should not be set, got %q", k, got)
				}
			}
		})
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	// Should not panic or error on missing file
	loadDotEnv("/nonexistent/path/.env")
}
