package utils

import (
	"bytes"
	"embed"
	"fmt"
	"log"
	"strings"
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
		templateInitError = err
		log.Printf("ERROR: Failed to parse templates (vLLM config feature will be disabled): %v", err)
	}
}

// vllmTemplatePreset is the template-friendly representation of a VLLMPreset
type vllmTemplatePreset struct {
	ModeTitle     string
	Optimizations []vllmTemplateOptimization
}

// vllmTemplateOptimization is the template-friendly representation of a VLLMOptimization
type vllmTemplateOptimization struct {
	OptimizationTitle string
	Hardware          string
	Description       string
	CLIArgsString     string
	HasCLIArgs        bool
	EnvVarsString     string
	HasEnvVars        bool
	Constraints       []string
	Recommendations   []string
}

// titleCase converts "online-serving" to "Online Serving"
func titleCase(s string) string {
	words := strings.Split(strings.ReplaceAll(s, "-", " "), " ")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// buildVLLMTemplateData transforms raw config into template-friendly data
func buildVLLMTemplateData(config *types.VLLMRecommendedConfig) struct {
	Presets []vllmTemplatePreset
} {
	var presets []vllmTemplatePreset
	for _, p := range config.Presets {
		tp := vllmTemplatePreset{
			ModeTitle: titleCase(p.Mode),
		}
		for _, opt := range p.Optimizations {
			to := vllmTemplateOptimization{
				OptimizationTitle: titleCase(opt.Optimization),
				Hardware:          opt.Hardware,
				Description:       opt.Description,
				CLIArgsString:     strings.Join(opt.CLIArgs, " \\\n    "),
				HasCLIArgs:        len(opt.CLIArgs) > 0,
				EnvVarsString:     strings.Join(opt.EnvVars, " \\\n    "),
				HasEnvVars:        len(opt.EnvVars) > 0,
				Recommendations:   opt.Recommendations,
			}
			for _, c := range opt.Constraints {
				to.Constraints = append(to.Constraints, c.FormatConstraint())
			}
			tp.Optimizations = append(tp.Optimizations, to)
		}
		presets = append(presets, tp)
	}
	return struct {
		Presets []vllmTemplatePreset
	}{Presets: presets}
}

// RenderVLLMConfigSection renders the vLLM recommended configurations section
// Returns empty string if config is nil or has no presets
func RenderVLLMConfigSection(config *types.VLLMRecommendedConfig) (string, error) {
	if config == nil || !config.HasPresets() {
		return "", nil
	}

	if templateInitError != nil {
		return "", fmt.Errorf("templates failed to initialize: %w", templateInitError)
	}

	if templateCache == nil {
		return "", fmt.Errorf("template cache not initialized")
	}

	tmpl := templateCache.Lookup("vllm-config.md.tmpl")
	if tmpl == nil {
		return "", fmt.Errorf("vllm-config template not found in cache")
	}

	data := buildVLLMTemplateData(config)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render vllm-config template: %w", err)
	}

	return buf.String(), nil
}
