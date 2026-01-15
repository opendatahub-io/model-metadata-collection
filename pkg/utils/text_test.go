package utils

import (
	"reflect"
	"testing"
)

func TestStripYAMLFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "content with frontmatter",
			input:    "---\nlicense: apache-2.0\ntags:\n  - llm\n---\n## Model Overview\nThis is the content.",
			expected: "## Model Overview\nThis is the content.",
		},
		{
			name:     "content without frontmatter",
			input:    "## Model Overview\nThis is the content.",
			expected: "## Model Overview\nThis is the content.",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only frontmatter",
			input:    "---\nlicense: apache-2.0\n---",
			expected: "",
		},
		{
			name:     "frontmatter with leading newlines after",
			input:    "---\nlicense: apache-2.0\n---\n\n\n## Content",
			expected: "## Content",
		},
		{
			name:     "malformed frontmatter no closing",
			input:    "---\nlicense: apache-2.0\nno closing marker",
			expected: "---\nlicense: apache-2.0\nno closing marker",
		},
		{
			name:     "frontmatter with whitespace before",
			input:    "  ---\nlicense: apache-2.0\n---\n## Content",
			expected: "## Content",
		},
		{
			name:     "content with triple dashes in body",
			input:    "---\nlicense: apache-2.0\n---\n## Content\nSome text --- with dashes --- here",
			expected: "## Content\nSome text --- with dashes --- here",
		},
		{
			name:     "only single triple dash",
			input:    "---",
			expected: "---",
		},
		{
			name:     "whitespace only content after frontmatter",
			input:    "---\nlicense: apache-2.0\n---\n   \n   ",
			expected: "   \n   ",
		},
		{
			name:     "nested yaml-like content after frontmatter",
			input:    "---\nlicense: apache-2.0\n---\n## Code Example\n```yaml\nname: test\n---\nvalue: 123\n```",
			expected: "## Code Example\n```yaml\nname: test\n---\nvalue: 123\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripYAMLFrontmatter(tt.input)
			if result != tt.expected {
				t.Errorf("StripYAMLFrontmatter() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestParseLanguageNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single language",
			input:    "english",
			expected: []string{"en"},
		},
		{
			name:     "multiple languages with comma",
			input:    "english, spanish, french",
			expected: []string{"en", "es", "fr"},
		},
		{
			name:     "multiple languages with and",
			input:    "english and spanish",
			expected: []string{"en", "es"},
		},
		{
			name:     "unknown language",
			input:    "klingon",
			expected: nil,
		},
		{
			name:     "mixed case",
			input:    "ENGLISH, Spanish",
			expected: []string{"en", "es"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseLanguageNames(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ParseLanguageNames() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateDescriptionFromModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic model name",
			input:    "RedHatAI/Llama-3.3-70B-Instruct",
			expected: "Llama 3.3 70B Instruct",
		},
		{
			name:     "quantized model",
			input:    "RedHatAI/granite-3.1-8b-base-quantized.w4a16",
			expected: "Granite 3.1 8b Base (w4a16 quantized)",
		},
		{
			name:     "whisper model",
			input:    "whisper-large-v2-quantized.w4a16",
			expected: "Whisper large v2 (w4a16 quantized)",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "fp8 dynamic model",
			input:    "meta-llama/Llama-3.1-8B-Instruct-FP8-dynamic",
			expected: "Llama 3.1 8B Instruct ((FP8) dynamic)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDescriptionFromModelName(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateDescriptionFromModelName() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "registry reference",
			input:    "registry.redhat.io/rhelai1/modelcar-granite-3-1-8b:1.0",
			expected: "granite-3-1-8b",
		},
		{
			name:     "huggingface reference",
			input:    "RedHatAI/Llama-3.3-70B-Instruct",
			expected: "llama-3-3-70b-instruct",
		},
		{
			name:     "mixed separators",
			input:    "test_model.name with spaces",
			expected: "test-model-name-with-spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeModelName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeModelName() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		minScore float64
		maxScore float64
	}{
		{
			name:     "exact match",
			s1:       "test-model",
			s2:       "test-model",
			minScore: 1.0,
			maxScore: 1.0,
		},
		{
			name:     "no similarity",
			s1:       "completely-different",
			s2:       "model-name",
			minScore: 0.0,
			maxScore: 0.0,
		},
		{
			name:     "partial similarity",
			s1:       "granite-3-1-8b",
			s2:       "granite-8b-model",
			minScore: 0.5,
			maxScore: 0.5,
		},
		{
			name:     "quantized model should match specific HF model better than generic",
			s1:       "registry.redhat.io/rhelai1/modelcar-llama-3-1-8b-instruct-quantized-w4a16:1.5",
			s2:       "RedHatAI/Meta-Llama-3.1-8B-Instruct-quantized.w4a16",
			minScore: 0.85,
			maxScore: 1.0,
		},
		{
			name:     "quantized model vs generic model (should score lower)",
			s1:       "registry.redhat.io/rhelai1/modelcar-llama-3-1-8b-instruct-quantized-w4a16:1.5",
			s2:       "RedHatAI/Llama-3.1-8B-Instruct",
			minScore: 0.5,
			maxScore: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculateSimilarity(tt.s1, tt.s2)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("CalculateSimilarity(%q, %q) = %f, expected between %f and %f",
					tt.s1, tt.s2, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestCalculateSimilarity_SpecificMatchesBetter(t *testing.T) {
	// This test ensures that RHOAIENG-38645 bug is fixed:
	// When matching "modelcar-llama-3-1-8b-instruct-quantized-w4a16",
	// it should prefer "Meta-Llama-3.1-8B-Instruct-quantized.w4a16"
	// over "Llama-3.1-8B-Instruct"

	container := "registry.redhat.io/rhelai1/modelcar-llama-3-1-8b-instruct-quantized-w4a16:1.5"
	correctMatch := "RedHatAI/Meta-Llama-3.1-8B-Instruct-quantized.w4a16"
	wrongMatch := "RedHatAI/Llama-3.1-8B-Instruct"

	correctScore := CalculateSimilarity(container, correctMatch)
	wrongScore := CalculateSimilarity(container, wrongMatch)

	if wrongScore >= correctScore {
		t.Errorf("Wrong match scored higher! correct=%f, wrong=%f. "+
			"Expected correct match to score higher for quantized model matching.",
			correctScore, wrongScore)
	}

	// The correct match should be significantly better (at least 10% higher)
	minDifference := 0.1
	if (correctScore - wrongScore) < minDifference {
		t.Errorf("Score difference too small: correct=%f, wrong=%f, diff=%f. "+
			"Expected at least %f difference.",
			correctScore, wrongScore, correctScore-wrongScore, minDifference)
	}
}

func TestCalculateSimilarity_Symmetry(t *testing.T) {
	// Test that similarity is symmetric (swapping s1 and s2 gives same result)
	// This ensures duplicate tokens are handled correctly
	testCases := []struct {
		name string
		s1   string
		s2   string
	}{
		{
			name: "duplicate tokens in first string",
			s1:   "llama-3-3-70b-instruct",
			s2:   "llama-3-70b-instruct",
		},
		{
			name: "duplicate tokens in second string",
			s1:   "granite-8b-model",
			s2:   "granite-3-1-3-8b",
		},
		{
			name: "complex model names",
			s1:   "registry.redhat.io/rhelai1/modelcar-llama-3-1-8b-instruct-quantized-w4a16:1.5",
			s2:   "RedHatAI/Meta-Llama-3.1-8B-Instruct-quantized.w4a16",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score1 := CalculateSimilarity(tc.s1, tc.s2)
			score2 := CalculateSimilarity(tc.s2, tc.s1)

			if score1 != score2 {
				t.Errorf("Similarity is not symmetric: CalculateSimilarity(%q, %q) = %f, but CalculateSimilarity(%q, %q) = %f",
					tc.s1, tc.s2, score1, tc.s2, tc.s1, score2)
			}
		})
	}
}
