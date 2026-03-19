package types

import (
	"strings"
	"testing"
)

func TestToolCallingConfig_HasToolCalling(t *testing.T) {
	tests := []struct {
		name   string
		config *ToolCallingConfig
		want   bool
	}{
		{
			name:   "nil config",
			config: nil,
			want:   false,
		},
		{
			name: "supported flag set",
			config: &ToolCallingConfig{
				Supported: true,
			},
			want: true,
		},
		{
			name: "has required CLI args",
			config: &ToolCallingConfig{
				RequiredCLIArgs: []string{"--config_format mistral"},
			},
			want: true,
		},
		{
			name: "has tool call parser",
			config: &ToolCallingConfig{
				ToolCallParser: "mistral",
			},
			want: true,
		},
		{
			name: "all fields set",
			config: &ToolCallingConfig{
				Supported:        true,
				RequiredCLIArgs:  []string{"--config_format mistral"},
				ToolCallParser:   "mistral",
				ChatTemplatePath: "examples/template.jinja",
			},
			want: true,
		},
		{
			name:   "no tool calling support",
			config: &ToolCallingConfig{},
			want:   false,
		},
		{
			name: "only chat template path set",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples/template.jinja",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.HasToolCalling(); got != tt.want {
				t.Errorf("HasToolCalling() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolCallingConfig_GetProcessedTemplatePath(t *testing.T) {
	tests := []struct {
		name   string
		config *ToolCallingConfig
		want   string
	}{
		{
			name:   "nil config",
			config: nil,
			want:   "",
		},
		{
			name: "examples path conversion (combined)",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples/chat_template.jinja",
			},
			want: "opt/app-root/template/chat_template.jinja",
		},
		{
			name: "examples path with nested directory",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples/mistral/chat_template.jinja",
			},
			want: "opt/app-root/template/mistral/chat_template.jinja",
		},
		{
			name: "split path and filename (Llama 3.1 style)",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples/",
				ChatTemplateFile: "tool_chat_template_llama3.1_json.jinja",
			},
			want: "opt/app-root/template/tool_chat_template_llama3.1_json.jinja",
		},
		{
			name: "split path and filename without trailing slash",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples",
				ChatTemplateFile: "template.jinja",
			},
			want: "opt/app-root/template/template.jinja",
		},
		{
			name: "filename only (no path)",
			config: &ToolCallingConfig{
				ChatTemplatePath: "",
				ChatTemplateFile: "template.jinja",
			},
			want: "opt/app-root/template/template.jinja",
		},
		{
			name: "filename None ignored",
			config: &ToolCallingConfig{
				ChatTemplatePath: "examples/template.jinja",
				ChatTemplateFile: "None",
			},
			want: "opt/app-root/template/template.jinja",
		},
		{
			name: "already processed path",
			config: &ToolCallingConfig{
				ChatTemplatePath: "opt/app-root/template/chat_template.jinja",
			},
			want: "opt/app-root/template/chat_template.jinja",
		},
		{
			name: "None value in path",
			config: &ToolCallingConfig{
				ChatTemplatePath: "None",
			},
			want: "",
		},
		{
			name: "empty path and no filename",
			config: &ToolCallingConfig{
				ChatTemplatePath: "",
			},
			want: "",
		},
		{
			name: "arbitrary path - extracts basename",
			config: &ToolCallingConfig{
				ChatTemplatePath: "some/other/path/template.jinja",
			},
			want: "opt/app-root/template/template.jinja",
		},
		{
			name: "absolute path - extracts basename",
			config: &ToolCallingConfig{
				ChatTemplatePath: "/usr/local/templates/template.jinja",
			},
			want: "opt/app-root/template/template.jinja",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.GetProcessedTemplatePath(); got != tt.want {
				t.Errorf("GetProcessedTemplatePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolCallingConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *ToolCallingConfig
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: false,
		},
		{
			name: "valid mistral parser",
			config: &ToolCallingConfig{
				ToolCallParser: "mistral",
			},
			wantError: false,
		},
		{
			name: "valid hermes parser",
			config: &ToolCallingConfig{
				ToolCallParser: "hermes",
			},
			wantError: false,
		},
		{
			name: "valid llama3_json parser",
			config: &ToolCallingConfig{
				ToolCallParser: "llama3_json",
			},
			wantError: false,
		},
		{
			name: "valid granite parser",
			config: &ToolCallingConfig{
				ToolCallParser: "granite",
			},
			wantError: false,
		},
		{
			name: "valid qwen parser",
			config: &ToolCallingConfig{
				ToolCallParser: "qwen",
			},
			wantError: false,
		},
		{
			name: "valid openai parser",
			config: &ToolCallingConfig{
				ToolCallParser: "openai",
			},
			wantError: false,
		},
		{
			name: "invalid parser",
			config: &ToolCallingConfig{
				ToolCallParser: "unknown-parser",
			},
			wantError: true,
			errorMsg:  "unknown tool_call_parser",
		},
		{
			name: "empty parser (valid)",
			config: &ToolCallingConfig{
				ToolCallParser: "",
			},
			wantError: false,
		},
		{
			name: "valid config with all fields",
			config: &ToolCallingConfig{
				Supported:        true,
				RequiredCLIArgs:  []string{"--config_format mistral"},
				ChatTemplateFile: "template.jinja",
				ChatTemplatePath: "examples/template.jinja",
				ToolCallParser:   "mistral",
			},
			wantError: false,
		},
		{
			name: "invalid parser in full config",
			config: &ToolCallingConfig{
				Supported:        true,
				RequiredCLIArgs:  []string{"--some-arg"},
				ChatTemplateFile: "template.jinja",
				ChatTemplatePath: "examples/template.jinja",
				ToolCallParser:   "invalid_parser_name",
			},
			wantError: true,
			errorMsg:  "unknown tool_call_parser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %v, expected to contain %q", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}
