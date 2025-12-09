package utils

import (
	"strings"
)

// licenseInfo contains comprehensive information about a license
type licenseInfo struct {
	Name string // Human-readable name
	URL  string // Canonical URL
}

// licenseRegistry is the single source of truth for license information
var licenseRegistry = map[string]licenseInfo{
	"apache-2.0":            {Name: "Apache 2.0", URL: "https://www.apache.org/licenses/LICENSE-2.0"},
	"mit":                   {Name: "MIT License", URL: "https://opensource.org/licenses/MIT"},
	"bsd-3-clause":          {Name: "BSD 3-Clause License", URL: "https://opensource.org/licenses/BSD-3-Clause"},
	"bsd-2-clause":          {Name: "BSD 2-Clause License", URL: "https://opensource.org/licenses/BSD-2-Clause"},
	"gpl-3.0":               {Name: "GPL 3.0", URL: "https://www.gnu.org/licenses/gpl-3.0.html"},
	"gpl-2.0":               {Name: "GPL 2.0", URL: "https://www.gnu.org/licenses/old-licenses/gpl-2.0.html"},
	"lgpl-3.0":              {Name: "LGPL 3.0", URL: "https://www.gnu.org/licenses/lgpl-3.0.html"},
	"lgpl-2.1":              {Name: "LGPL 2.1", URL: "https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html"},
	"cc-by-4.0":             {Name: "Creative Commons Attribution 4.0", URL: "https://creativecommons.org/licenses/by/4.0/"},
	"cc-by-sa-4.0":          {Name: "Creative Commons Attribution-ShareAlike 4.0", URL: "https://creativecommons.org/licenses/by-sa/4.0/"},
	"cc-by-nc-4.0":          {Name: "Creative Commons Attribution-NonCommercial 4.0", URL: "https://creativecommons.org/licenses/by-nc/4.0/"},
	"cc0-1.0":               {Name: "Creative Commons Zero v1.0 Universal", URL: "https://creativecommons.org/publicdomain/zero/1.0/"},
	"unlicense":             {Name: "The Unlicense", URL: "https://unlicense.org/"},
	"llama2":                {Name: "Llama 2 Community License", URL: "https://github.com/facebookresearch/llama/blob/main/LICENSE"},
	"llama3":                {Name: "Llama 3 Community License", URL: "https://github.com/meta-llama/llama-models/blob/main/models/llama3/LICENSE"},
	"llama3.1":              {Name: "Llama 3.1 Community License", URL: "https://github.com/meta-llama/llama-models/blob/main/models/llama3_1/LICENSE"},
	"llama3.2":              {Name: "Llama 3.2 Community License", URL: "https://github.com/meta-llama/llama-models/blob/main/models/llama3_2/LICENSE"},
	"llama3.3":              {Name: "Llama 3.3 Community License", URL: "https://github.com/meta-llama/llama-models/blob/main/models/llama3_3/LICENSE"},
	"llama4":                {Name: "Llama 4 Community License", URL: "https://github.com/meta-llama/llama-models/blob/main/models/llama4/LICENSE"},
	"bigscience-openrail-m": {Name: "BigScience OpenRAIL-M", URL: "https://huggingface.co/spaces/bigscience/license"},
	"openrail":              {Name: "OpenRAIL", URL: "https://www.licenses.ai/ai-licenses"},
	"gemma":                 {Name: "Gemma", URL: "https://ai.google.dev/gemma/terms"},
}

// GetLicenseURL returns the canonical URL for well-known licenses
func GetLicenseURL(licenseID string) string {
	licenseID = strings.ToLower(strings.TrimSpace(licenseID))

	if info, exists := licenseRegistry[licenseID]; exists {
		return info.URL
	}

	return ""
}

// GetHumanReadableLicenseName returns the human-readable name for well-known licenses
func GetHumanReadableLicenseName(licenseID string) string {
	licenseID = strings.TrimSpace(licenseID)

	if info, exists := licenseRegistry[strings.ToLower(licenseID)]; exists {
		return info.Name
	}

	// Return original value if no mapping found (graceful fallback)
	return strings.TrimSpace(licenseID)
}
