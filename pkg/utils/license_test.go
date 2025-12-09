package utils

import (
	"testing"
)

func TestGetLicenseURL(t *testing.T) {
	tests := []struct {
		name      string
		licenseID string
		expected  string
	}{
		{
			name:      "apache-2.0",
			licenseID: "apache-2.0",
			expected:  "https://www.apache.org/licenses/LICENSE-2.0",
		},
		{
			name:      "MIT license",
			licenseID: "mit",
			expected:  "https://opensource.org/licenses/MIT",
		},
		{
			name:      "case insensitive",
			licenseID: "APACHE-2.0",
			expected:  "https://www.apache.org/licenses/LICENSE-2.0",
		},
		{
			name:      "whitespace handling",
			licenseID: "  apache-2.0  ",
			expected:  "https://www.apache.org/licenses/LICENSE-2.0",
		},
		{
			name:      "unknown license",
			licenseID: "unknown-license",
			expected:  "",
		},
		{
			name:      "llama license",
			licenseID: "llama3.1",
			expected:  "https://github.com/meta-llama/llama-models/blob/main/models/llama3_1/LICENSE",
		},
		{
			name:      "empty string",
			licenseID: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLicenseURL(tt.licenseID)
			if result != tt.expected {
				t.Errorf("GetLicenseURL() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestGetHumanReadableLicenseName(t *testing.T) {
	tests := []struct {
		name      string
		licenseID string
		expected  string
	}{
		{
			name:      "apache-2.0",
			licenseID: "apache-2.0",
			expected:  "Apache 2.0",
		},
		{
			name:      "MIT license",
			licenseID: "mit",
			expected:  "MIT License",
		},
		{
			name:      "BSD 3-clause",
			licenseID: "bsd-3-clause",
			expected:  "BSD 3-Clause License",
		},
		{
			name:      "GPL 3.0",
			licenseID: "gpl-3.0",
			expected:  "GPL 3.0",
		},
		{
			name:      "case insensitive",
			licenseID: "APACHE-2.0",
			expected:  "Apache 2.0",
		},
		{
			name:      "whitespace handling",
			licenseID: "  mit  ",
			expected:  "MIT License",
		},
		{
			name:      "llama license",
			licenseID: "llama3.1",
			expected:  "Llama 3.1 Community License",
		},
		{
			name:      "creative commons",
			licenseID: "cc-by-4.0",
			expected:  "Creative Commons Attribution 4.0",
		},
		{
			name:      "unknown license fallback",
			licenseID: "unknown-license",
			expected:  "unknown-license",
		},
		{
			name:      "empty string fallback",
			licenseID: "",
			expected:  "",
		},
		{
			name:      "whitespace only fallback",
			licenseID: "   ",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHumanReadableLicenseName(tt.licenseID)
			if result != tt.expected {
				t.Errorf("GetHumanReadableLicenseName() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
