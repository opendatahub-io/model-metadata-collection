package config

import (
	"regexp"
	"strings"
	"testing"
)

func TestSupportedModelFamilies_Alphabetical(t *testing.T) {
	// Ensure model families are sorted alphabetically for maintainability
	for i := 0; i < len(SupportedModelFamilies)-1; i++ {
		if SupportedModelFamilies[i] >= SupportedModelFamilies[i+1] {
			t.Errorf("Model families not in alphabetical order: %s should come before %s",
				SupportedModelFamilies[i+1], SupportedModelFamilies[i])
		}
	}
}

func TestSupportedModelFamilies_NoDuplicates(t *testing.T) {
	// Check for duplicate entries
	seen := make(map[string]bool)
	for _, family := range SupportedModelFamilies {
		if seen[family] {
			t.Errorf("Duplicate model family found: %s", family)
		}
		seen[family] = true
	}
}

func TestSupportedModelFamilies_ValidFormat(t *testing.T) {
	// Ensure all family names are lowercase and alphanumeric
	validFormat := regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	for _, family := range SupportedModelFamilies {
		if !validFormat.MatchString(family) {
			t.Errorf("Invalid family name format: %s (must be lowercase alphanumeric)", family)
		}
	}
}

func TestGetModelFamilyRegexPattern(t *testing.T) {
	pattern := GetModelFamilyRegexPattern()

	// Verify pattern is valid regex
	_, err := regexp.Compile(pattern)
	if err != nil {
		t.Errorf("Invalid regex pattern: %v", err)
		return
	}

	// Extract the alternation group from the pattern
	// Expected format: (family1|family2|...)-(\w?\d+)-(\d+)
	// We need to extract the families from the first group
	alternationRegex := regexp.MustCompile(`^\(([^)]+)\)`)
	matches := alternationRegex.FindStringSubmatch(pattern)
	if len(matches) < 2 {
		t.Fatalf("Could not extract alternation group from pattern: %s", pattern)
	}

	// Split on pipe to get individual family tokens
	patternFamilies := strings.Split(matches[1], "|")

	// Convert to maps for comparison
	expectedFamilies := make(map[string]bool)
	for _, family := range SupportedModelFamilies {
		expectedFamilies[family] = true
	}

	actualFamilies := make(map[string]bool)
	for _, family := range patternFamilies {
		actualFamilies[family] = true
	}

	// Verify all expected families are in the pattern (no missing families)
	for _, family := range SupportedModelFamilies {
		if !actualFamilies[family] {
			t.Errorf("Regex pattern missing family: %s", family)
		}
	}

	// Verify no extra families in the pattern (no unexpected families)
	for _, family := range patternFamilies {
		if !expectedFamilies[family] {
			t.Errorf("Regex pattern contains unexpected family: %s", family)
		}
	}

	// Verify counts match (catches duplicates)
	if len(patternFamilies) != len(SupportedModelFamilies) {
		t.Errorf("Pattern family count mismatch: got %d, expected %d",
			len(patternFamilies), len(SupportedModelFamilies))
	}
}

func TestGetModelFamilyRegex(t *testing.T) {
	regex := GetModelFamilyRegex()

	// Test cases covering all model families and version patterns
	testCases := []struct {
		name     string
		input    string
		expected []string // [full match, family, version, subversion]
	}{
		{
			name:     "granite standard version",
			input:    "granite-3-1",
			expected: []string{"granite-3-1", "granite", "3", "1"},
		},
		{
			name:     "llama standard version",
			input:    "llama-3-3",
			expected: []string{"llama-3-3", "llama", "3", "3"},
		},
		{
			name:     "minimax prefixed version",
			input:    "minimax-m2-5",
			expected: []string{"minimax-m2-5", "minimax", "m2", "5"},
		},
		{
			name:     "mistral standard version",
			input:    "mistral-7-1",
			expected: []string{"mistral-7-1", "mistral", "7", "1"},
		},
		{
			name:     "qwen standard version",
			input:    "qwen-2-5",
			expected: []string{"qwen-2-5", "qwen", "2", "5"},
		},
		{
			name:     "phi standard version",
			input:    "phi-3-5",
			expected: []string{"phi-3-5", "phi", "3", "5"},
		},
		{
			name:     "gemma standard version",
			input:    "gemma-2-9",
			expected: []string{"gemma-2-9", "gemma", "2", "9"},
		},
		{
			name:     "deepseek standard version",
			input:    "deepseek-3-0",
			expected: []string{"deepseek-3-0", "deepseek", "3", "0"},
		},
		{
			name:     "kimi standard version",
			input:    "kimi-1-5",
			expected: []string{"kimi-1-5", "kimi", "1", "5"},
		},
		{
			name:     "mixtral standard version",
			input:    "mixtral-8-7",
			expected: []string{"mixtral-8-7", "mixtral", "8", "7"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := regex.FindStringSubmatch(tc.input)
			if len(matches) == 0 {
				t.Errorf("Pattern did not match input %q", tc.input)
				return
			}

			if len(matches) != 4 {
				t.Errorf("Expected 4 groups, got %d: %v", len(matches), matches)
				return
			}

			for i, expected := range tc.expected {
				if matches[i] != expected {
					t.Errorf("Group %d: got %q, expected %q", i, matches[i], expected)
				}
			}
		})
	}
}

func TestGetModelFamilyRegex_NoMatch(t *testing.T) {
	regex := GetModelFamilyRegex()

	// Cases that should NOT match
	noMatchCases := []string{
		"unknown-3-1",       // Unsupported family
		"granite-abc-1",     // Invalid version format (more than 1 letter)
		"llama-3",           // Missing subversion
		"mistral",           // Missing version and subversion
		"granite-3-1-extra", // Extra components after pattern
	}

	for _, input := range noMatchCases {
		t.Run("no_match_"+input, func(t *testing.T) {
			// The regex might match a substring, so we check if it matches the specific pattern
			matches := regex.FindStringSubmatch(input)
			if len(matches) > 0 && matches[0] == input {
				t.Errorf("Pattern should not fully match %q, but got: %v", input, matches)
			}
		})
	}
}

func TestIsModelFamily(t *testing.T) {
	// Test all supported families
	for _, family := range SupportedModelFamilies {
		t.Run("supported_"+family, func(t *testing.T) {
			if !IsModelFamily(family) {
				t.Errorf("IsModelFamily(%q) = false, expected true", family)
			}
		})
	}

	// Test unsupported families
	unsupported := []string{
		"unknown",
		"claude",
		"gpt",
		"bert",
		"",
	}

	for _, family := range unsupported {
		t.Run("unsupported_"+family, func(t *testing.T) {
			if IsModelFamily(family) {
				t.Errorf("IsModelFamily(%q) = true, expected false", family)
			}
		})
	}
}

func TestModelFamilyRegex_Singleton(t *testing.T) {
	// Verify GetModelFamilyRegex returns the same instance (singleton pattern)
	regex1 := GetModelFamilyRegex()
	regex2 := GetModelFamilyRegex()

	if regex1 != regex2 {
		t.Error("GetModelFamilyRegex should return singleton instance")
	}
}

func TestModelFamilyRegex_ConcurrentAccess(t *testing.T) {
	// Verify GetModelFamilyRegex is safe for concurrent access (using sync.Once)
	// This test ensures no data races occur when multiple goroutines call GetModelFamilyRegex

	const goroutines = 100
	results := make([]*regexp.Regexp, goroutines)
	done := make(chan bool, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(index int) {
			results[index] = GetModelFamilyRegex()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Verify all goroutines got the same regex instance
	firstRegex := results[0]
	for i := 1; i < goroutines; i++ {
		if results[i] != firstRegex {
			t.Errorf("Concurrent access returned different regex instances: %p vs %p", firstRegex, results[i])
		}
	}

	// Verify the regex actually works
	if !firstRegex.MatchString("granite-3-1") {
		t.Error("Regex should match valid model family pattern")
	}
}
