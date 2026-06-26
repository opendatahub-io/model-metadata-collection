package huggingface

import (
	"testing"
)

func TestParseVersionFromTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "semver version v1.0",
			title:    "Red Hat AI validated models - v1.0",
			expected: "v1.0",
		},
		{
			name:     "semver version v2.1",
			title:    "Red Hat AI validated models - v2.1",
			expected: "v2.1",
		},
		{
			name:     "date pattern May 2025",
			title:    "Red Hat AI validated models - May 2025",
			expected: "v2025.05",
		},
		{
			name:     "date pattern September 2025",
			title:    "Red Hat AI validated models - September 2025",
			expected: "v2025.09",
		},
		{
			name:     "date pattern January 2026",
			title:    "Red Hat AI validated models - January 2026",
			expected: "v2026.01",
		},
		{
			name:     "date pattern February 2026",
			title:    "Red Hat AI validated models - February 2026",
			expected: "v2026.02",
		},
		{
			name:     "granite quantized collection",
			title:    "Granite Quantized",
			expected: "v1.0-granite-quantized",
		},
		{
			name:     "granite quantized with different casing",
			title:    "GRANITE QUANTIZED",
			expected: "v1.0-granite-quantized",
		},
		{
			name:     "granite quantized with extra words",
			title:    "Red Hat AI Granite Quantized Models",
			expected: "v1.0-granite-quantized",
		},
		{
			name:     "embedding models collection",
			title:    "Embedding Models",
			expected: "v1.0-embedding-models",
		},
		{
			name:     "embedding models with different casing",
			title:    "EMBEDDING MODELS",
			expected: "v1.0-embedding-models",
		},
		{
			name:     "embedding models with extra words",
			title:    "Red Hat AI Embedding Models Collection",
			expected: "v1.0-embedding-models",
		},
		{
			name:     "other ea2 models collection",
			title:    "Other EA2 Models",
			expected: "v1.0-other-ea2-models",
		},
		{
			name:     "other ea2 models with different casing",
			title:    "OTHER EA2 MODELS",
			expected: "v1.0-other-ea2-models",
		},
		{
			name:     "other ea2 models with extra words",
			title:    "Red Hat AI Other EA2 Models Collection",
			expected: "v1.0-other-ea2-models",
		},
		{
			name:     "embedding takes precedence over other ea2",
			title:    "Other Embedding EA2 Models",
			expected: "v1.0-embedding-models",
		},
		{
			name:     "no matching pattern",
			title:    "Some random collection",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVersionFromTitle(tt.title)
			if result != tt.expected {
				t.Errorf("parseVersionFromTitle(%q) = %q, expected %q", tt.title, result, tt.expected)
			}
		})
	}
}
