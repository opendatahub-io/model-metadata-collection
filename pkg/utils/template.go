package utils

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"text/template"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templateCache *template.Template
var templateInitError error

func init() {
	var err error
	templateCache, err = template.ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		// Templates are required for tool-calling feature
		// Store error for later checking and log prominently
		templateInitError = err
		log.Printf("ERROR: Failed to parse templates (tool-calling feature will be disabled): %v", err)
	}
}

// RenderToolCallingSection renders the tool-calling documentation section
// Returns empty string if config is nil or has no tool calling support
func RenderToolCallingSection(config *types.ToolCallingConfig, modelName string) (string, error) {
	if config == nil || !config.HasToolCalling() {
		return "", nil // No tool calling support, return empty string
	}

	// Check if templates failed to initialize
	if templateInitError != nil {
		return "", fmt.Errorf("templates failed to initialize: %w", templateInitError)
	}

	if templateCache == nil {
		return "", fmt.Errorf("template cache not initialized (this should not happen)")
	}

	tmpl := templateCache.Lookup("tool-calling.md.tmpl")
	if tmpl == nil {
		return "", fmt.Errorf("tool-calling template not found in cache")
	}

	data := struct {
		ModelName       string
		Config          *types.ToolCallingConfig
		ProcessedPath   string
		HasTemplateFile bool
		HasCLIArgs      bool
	}{
		ModelName:       modelName,
		Config:          config,
		ProcessedPath:   config.GetProcessedTemplatePath(),
		HasTemplateFile: config.ChatTemplateFile != "" && config.ChatTemplateFile != "None",
		HasCLIArgs:      len(config.RequiredCLIArgs) > 0,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}
