package huggingface

import (
	"regexp"
	"slices"
	"strings"
)

// ParseTagsForStructuredData extracts structured metadata from HuggingFace tags
func ParseTagsForStructuredData(tags []string) (languages []string, license string, tasks []string) {
	// Known language codes
	languageCodes := map[string]bool{
		"en": true, "fr": true, "de": true, "es": true, "it": true, "pt": true, "ru": true,
		"zh": true, "ja": true, "ko": true, "ar": true, "hi": true, "nl": true, "sv": true,
		"da": true, "no": true, "fi": true, "pl": true, "cs": true, "hu": true, "tr": true,
		"he": true, "th": true, "vi": true, "id": true, "ms": true, "tl": true, "sw": true,
	}

	// Known task types mapped to standardized format
	taskTypes := map[string]string{
		"text-generation":              "text-generation",
		"text-classification":          "text-classification",
		"text-to-text-generation":      "text-generation",
		"translation":                  "text-generation",
		"summarization":                "text-generation",
		"question-answering":           "question-answering",
		"conversational":               "text-generation",
		"text-to-speech":               "text-generation",
		"automatic-speech-recognition": "text-generation",
		"image-classification":         "image-classification",
		"image-to-text":                "image-to-text",
		"text-to-image":                "text-generation",
		"feature-extraction":           "text-generation",
		"sentence-similarity":          "sentence-similarity",
		"zero-shot-classification":     "text-classification",
		"token-classification":         "text-classification",
		"fill-mask":                    "text-generation",
		"multiple-choice":              "question-answering",
		"table-question-answering":     "question-answering",
		"visual-question-answering":    "question-answering",
		"any-to-any":                   "any-to-any",
		"image-text-to-text":           "image-text-to-text",
		"image-to-image":               "image-to-image",
		"text-ranking":                 "text-ranking",
		"text-to-video":                "text-to-video",
		"video-to-video":               "video-to-video",
	}

	// Known license patterns (for detecting license names in tags)
	licensePatterns := map[string]string{
		"llama2":       "llama2",
		"llama3":       "llama3",
		"llama3.1":     "llama3.1",
		"llama3.2":     "llama3.2",
		"llama3.3":     "llama3.3",
		"llama4":       "llama4",
		"apache-2.0":   "apache-2.0",
		"mit":          "mit",
		"gpl-3.0":      "gpl-3.0",
		"bsd-3-clause": "bsd-3-clause",
	}

	for _, tag := range tags {
		tag = strings.TrimSpace(strings.ToLower(tag))

		// Extract license from license: prefix
		if after, ok := strings.CutPrefix(tag, "license:"); ok {
			extractedLicense := after
			// Only use it if it's not "other" - we'll try to find a better license below
			if extractedLicense != "other" {
				license = extractedLicense
			}
			continue
		}

		// Check if the tag itself is a license name (for cases where license:other but llama4 tag exists)
		if licenseValue, isLicense := licensePatterns[tag]; isLicense {
			// Prefer specific license names over "other"
			if license == "" || license == "other" {
				license = licenseValue
			}
			continue
		}

		// Check if it's a language code
		if languageCodes[tag] {
			languages = append(languages, tag)
			continue
		}

		// Check if it's a task type and normalize it
		if normalizedTask, exists := taskTypes[tag]; exists {
			// Avoid duplicates
			found := slices.Contains(tasks, normalizedTask)
			if !found {
				tasks = append(tasks, normalizedTask)
			}
			continue
		}
	}

	return languages, license, tasks
}

// InferTasksFromReadme attempts to infer model tasks from README content
func InferTasksFromReadme(readme string) []string {
	var tasks []string

	// Pattern to detect text input/output architecture - flexible patterns
	patterns := []string{
		// Pattern 1: **Input:** Text ... **Output:** Text (with potential content in between)
		`(?i)\*\*Input:\*\*\s*Text.*?\*\*Output:\*\*\s*Text`,
		// Pattern 2: - **Input:** Text and - **Output:** Text (separate lines)
		`(?i)-\s*\*\*Input:\*\*\s*Text[\s\S]*?-\s*\*\*Output:\*\*\s*Text`,
		// Pattern 3: Input: Text and Output: Text (without asterisks)
		`(?i)Input:\s*Text[\s\S]*?Output:\s*Text`,
	}

	for _, pattern := range patterns {
		textIOPattern := regexp.MustCompile(pattern)
		if textIOPattern.MatchString(readme) {
			tasks = append(tasks, "text-generation")
			break // Found one pattern, no need to check others
		}
	}

	return tasks
}

// FilterTagsForCleanTagList filters HuggingFace repository tags to only include clean tags
// suitable for the tags field, excluding language codes, arxiv references, and other metadata
func FilterTagsForCleanTagList(tags []string) []string {
	var filteredTags []string

	// Known language codes to exclude (comprehensive list)
	languageCodes := map[string]bool{
		// Common ISO 639-1 codes
		"en": true, "fr": true, "de": true, "es": true, "it": true, "pt": true, "ru": true,
		"zh": true, "ja": true, "ko": true, "ar": true, "hi": true, "nl": true, "sv": true,
		"da": true, "no": true, "fi": true, "pl": true, "cs": true, "hu": true, "tr": true,
		"he": true, "th": true, "vi": true, "id": true, "ms": true, "tl": true, "sw": true,
		// Additional language codes found in repository tags
		"bg": true, "el": true, "fa": true, "ro": true, "sr": true, "uk": true, "ur": true,
		"zsm": true, "nld": true, "ca": true, "eu": true, "gl": true, "hr": true, "lv": true,
		"lt": true, "mk": true, "mt": true, "sk": true, "sl": true, "et": true, "bn": true,
		"ne": true, "ta": true, "te": true, "ml": true, "si": true, "my": true, "km": true,
		"lo": true, "ka": true, "am": true, "is": true, "ga": true, "cy": true, "sq": true,
		"be": true, "bs": true, "eo": true, "fo": true, "fy": true, "gd": true, "lb": true,
		"mn": true, "nn": true, "oc": true, "rm": true, "sc": true, "tt": true, "uz": true,
		"wo": true, "yo": true, "zu": true, "af": true, "az": true, "hy": true,
		"kk": true, "ky": true, "tg": true, "tk": true, "ug": true, "xh": true,
	}

	// Known task types to exclude (these should go in tasks field, not tags)
	taskTypes := map[string]bool{
		"text-generation": true, "text-classification": true, "text-to-text-generation": true,
		"translation": true, "summarization": true, "question-answering": true,
		"conversational": true, "text-to-speech": true, "automatic-speech-recognition": true,
		"image-classification": true, "image-to-text": true, "text-to-image": true,
		"feature-extraction": true, "sentence-similarity": true, "zero-shot-classification": true,
		"token-classification": true, "fill-mask": true, "multiple-choice": true,
		"table-question-answering": true, "visual-question-answering": true,
		"any-to-any": true, "image-text-to-text": true, "image-to-image": true,
		"text-ranking": true, "text-to-video": true, "video-to-video": true,
		"text-generation-inference": true,
	}

	for _, tag := range tags {
		originalTag := strings.TrimSpace(tag)
		lowerTag := strings.ToLower(originalTag)

		// Skip if empty
		if originalTag == "" {
			continue
		}

		// Skip language codes
		if languageCodes[lowerTag] {
			continue
		}

		// Skip task types (these should be in tasks field)
		if taskTypes[lowerTag] {
			continue
		}

		// Skip arxiv references
		if strings.HasPrefix(lowerTag, "arxiv:") {
			continue
		}

		// Skip base_model references
		if strings.HasPrefix(lowerTag, "base_model:") {
			continue
		}

		// Skip license references (these should be in license field)
		if strings.HasPrefix(lowerTag, "license:") {
			continue
		}

		// Skip region references
		if strings.HasPrefix(lowerTag, "region:") {
			continue
		}

		// Include everything else as legitimate tags
		filteredTags = append(filteredTags, originalTag)
	}

	return filteredTags
}
