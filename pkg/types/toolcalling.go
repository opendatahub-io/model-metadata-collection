package types

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validToolCallParsers defines the set of known vLLM tool call parsers
// Reference: https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html#tool-calling
var validToolCallParsers = map[string]bool{
	"mistral":       true,
	"hermes":        true,
	"llama3_json":   true,
	"internlm2":     true,
	"granite":       true,
	"jamba":         true,
	"llama":         true,
	"functionary":   true,
	"qwen":          true,
	"metamath":      true,
	"openai":        true,
	"internlm":      true,
	"deepseek_v2.5": true,
}

// ToolCallingConfig represents tool-calling configuration from YAML frontmatter
type ToolCallingConfig struct {
	Supported        bool     `yaml:"tool_calling_supported"`
	RequiredCLIArgs  []string `yaml:"required_cli_args"`
	ChatTemplateFile string   `yaml:"chat_template_file_name"`
	ChatTemplatePath string   `yaml:"chat_template_path"`
	ToolCallParser   string   `yaml:"tool_call_parser"`
}

// HasToolCalling returns true if the model supports tool calling
func (tc *ToolCallingConfig) HasToolCalling() bool {
	if tc == nil {
		return false
	}
	return tc.Supported || len(tc.RequiredCLIArgs) > 0 || tc.ToolCallParser != ""
}

// Validate checks that the tool-calling configuration is valid
func (tc *ToolCallingConfig) Validate() error {
	if tc == nil {
		return nil
	}

	// Validate tool call parser if specified
	if tc.ToolCallParser != "" && !validToolCallParsers[tc.ToolCallParser] {
		return fmt.Errorf("unknown tool_call_parser: %q (known parsers: mistral, hermes, llama3_json, internlm2, granite, jamba, llama, functionary, qwen, metamath, openai, internlm, deepseek_v2.5)", tc.ToolCallParser)
	}

	return nil
}

// GetProcessedTemplatePath converts chat_template_path from examples/* to opt/app-root/template/*
// This conversion is always applied for RHOAI/OpenShift AI deployments
//
// Handles two cases:
// 1. Separate path + filename: chat_template_path="examples/" + chat_template_file_name="template.jinja"
// 2. Combined path: chat_template_path="examples/template.jinja"
func (tc *ToolCallingConfig) GetProcessedTemplatePath() string {
	if tc == nil {
		return ""
	}

	// Build the full path by combining directory and filename if both are present
	fullPath := tc.ChatTemplatePath
	if tc.ChatTemplateFile != "" && tc.ChatTemplateFile != "None" {
		// If we have a filename, combine it with the path
		if fullPath != "" && fullPath != "None" {
			// Ensure path ends with / before combining
			if !strings.HasSuffix(fullPath, "/") {
				fullPath += "/"
			}
			fullPath = fullPath + tc.ChatTemplateFile
		} else {
			// No path specified, just use the filename
			fullPath = tc.ChatTemplateFile
		}
	}

	// If we still don't have a valid path, return empty
	if fullPath == "" || fullPath == "None" {
		return ""
	}

	// Always convert examples/* to opt/app-root/template/*
	if strings.HasPrefix(fullPath, "examples/") {
		return strings.Replace(fullPath, "examples/", "opt/app-root/template/", 1)
	}

	// Already in correct format
	if strings.HasPrefix(fullPath, "opt/app-root/template/") {
		return fullPath
	}

	// Default conversion for any other path format
	return "opt/app-root/template/" + filepath.Base(fullPath)
}
