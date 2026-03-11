package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// SupportedModelFamilies defines all recognized model families for metadata enrichment.
// This is the SINGLE SOURCE OF TRUTH for model families used across the codebase.
//
// CRITICAL: When adding a new model family:
// 1. Add the family name to this slice (alphabetically sorted)
// 2. Update tests in pkg/utils/text_test.go
// 3. Update tests in internal/enrichment/enrichment_test.go (if exists)
// 4. Run `make test` to ensure consistency checks pass
//
// Used by:
// - internal/enrichment/enrichment.go: extractModelFamily() for cross-family matching prevention
// - pkg/utils/text.go: NormalizeModelName() for version normalization regex
var SupportedModelFamilies = []string{
	"deepseek",
	"gemma",
	"granite",
	"kimi",
	"llama",
	"minimax",
	"mistral",
	"mixtral",
	"phi",
	"qwen",
}

// IsModelFamily checks if a token matches any supported model family
func IsModelFamily(token string) bool {
	return slices.Contains(SupportedModelFamilies, token)
}

// GetModelFamilyRegexPattern returns the regex pattern for matching model families
// in version normalization. The pattern matches: (family)-(version)-(subversion)
//
// Examples:
//   - "granite-3-1" → matches with family="granite", version="3", subversion="1"
//   - "minimax-m2-5" → matches with family="minimax", version="m2", subversion="5"
//   - "llama-3-3" → matches with family="llama", version="3", subversion="3"
//
// The version group uses (\w?\d+) to support both:
//   - Standard versions: "3", "8", "70" (numeric only)
//   - Prefixed versions: "m2", "v3", "a1" (single letter + numeric)
//
// IMPORTANT: This pattern is compiled into a regex in pkg/utils/text.go
func GetModelFamilyRegexPattern() string {
	familiesPattern := strings.Join(SupportedModelFamilies, "|")
	return fmt.Sprintf(`(%s)-(\w?\d+)-(\d+)`, familiesPattern)
}

// GetModelFamilyRegex returns a compiled regex for model family version matching.
// Uses sync.Once to ensure thread-safe lazy initialization for concurrent access.
var (
	modelFamilyRegex *regexp.Regexp
	modelFamilyOnce  sync.Once
)

func GetModelFamilyRegex() *regexp.Regexp {
	modelFamilyOnce.Do(func() {
		modelFamilyRegex = regexp.MustCompile(GetModelFamilyRegexPattern())
	})
	return modelFamilyRegex
}
