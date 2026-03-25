package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
)

// MetadataReport represents a comprehensive report of metadata completeness and sources
type MetadataReport struct {
	GeneratedAt time.Time     `yaml:"generated_at"`
	Summary     ReportSummary `yaml:"summary"`
	Models      []ModelReport `yaml:"models"`
}

// ReportSummary provides high-level statistics
type ReportSummary struct {
	TotalModels       int                     `yaml:"total_models"`
	FieldCompleteness map[string]Completeness `yaml:"field_completeness"`
	DataSources       map[string]int          `yaml:"data_sources"`
}

// Completeness tracks how many models have data for each field
type Completeness struct {
	Populated  int     `yaml:"populated"`
	Null       int     `yaml:"null"`
	Percentage float64 `yaml:"percentage"`
}

// ModelReport contains metadata analysis for a single model
type ModelReport struct {
	Name            string                 `yaml:"name"`
	Provider        string                 `yaml:"provider,omitempty"`
	Fields          map[string]FieldStatus `yaml:"fields"`
	MissingFields   []string               `yaml:"missing_fields,omitempty"`
	DataSources     map[string]int         `yaml:"data_sources"`
	SourceBreakdown SourceBreakdown        `yaml:"source_breakdown,omitempty"`
}

// SourceBreakdown provides detailed source analysis
type SourceBreakdown struct {
	ModelcardYAML    int `yaml:"modelcard_yaml"`
	ModelcardRegex   int `yaml:"modelcard_regex"`
	HuggingfaceYAML  int `yaml:"huggingface_yaml"`
	HuggingfaceTags  int `yaml:"huggingface_tags"`
	HuggingfaceRegex int `yaml:"huggingface_regex"`
	Registry         int `yaml:"registry"`
	Generated        int `yaml:"generated"`
	Other            int `yaml:"other"`
}

// FieldStatus indicates the source and status of a metadata field
type FieldStatus struct {
	Value           interface{} `yaml:"value,omitempty"`
	Source          string      `yaml:"source"`
	DetectionMethod string      `yaml:"detection_method"`
	IsNull          bool        `yaml:"is_null"`
	IsEmpty         bool        `yaml:"is_empty,omitempty"`
}

// GenerateMetadataReport creates a comprehensive metadata report
func GenerateMetadataReport(catalogPath, outputDir, reportDir string) error {
	// Read the catalog file
	catalog, err := readCatalog(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %w", err)
	}

	// Load enrichment data for each model
	enrichmentData, err := loadEnrichmentData(outputDir, catalog.Models)
	if err != nil {
		return fmt.Errorf("failed to load enrichment data: %w", err)
	}

	// Generate the report
	report := generateReport(catalog, enrichmentData)

	// Write markdown report
	markdownPath := filepath.Join(reportDir, "metadata-report.md")
	if err := writeMarkdownReport(report, markdownPath); err != nil {
		return fmt.Errorf("failed to write markdown report: %w", err)
	}

	// Write YAML report for programmatic use
	yamlPath := filepath.Join(reportDir, "metadata-report.yaml")
	if err := writeYAMLReport(report, yamlPath); err != nil {
		return fmt.Errorf("failed to write YAML report: %w", err)
	}

	fmt.Printf("Metadata reports generated:\n")
	fmt.Printf("  Markdown: %s\n", markdownPath)
	fmt.Printf("  YAML: %s\n", yamlPath)

	return nil
}

// readCatalog reads and parses the models catalog
func readCatalog(catalogPath string) (*types.ModelsCatalog, error) {
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, err
	}

	var catalog types.ModelsCatalog
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}

	return &catalog, nil
}

// SimpleEnrichmentData represents the actual structure of enrichment.yaml files
type SimpleEnrichmentData struct {
	HuggingFaceModel string            `yaml:"huggingface_model"`
	HuggingFaceURL   string            `yaml:"huggingface_url"`
	MatchConfidence  string            `yaml:"match_confidence"`
	DataSources      map[string]string `yaml:"data_sources"`
}

// loadEnrichmentData loads enrichment data for all models
func loadEnrichmentData(outputDir string, models []types.CatalogMetadata) (map[string]*SimpleEnrichmentData, error) {
	enrichmentData := make(map[string]*SimpleEnrichmentData)

	// Build a map of model names to enrichment data by scanning all directories
	modelDirs, err := filepath.Glob(filepath.Join(outputDir, "*"))
	if err != nil {
		return enrichmentData, err
	}

	// Map each enrichment file to its model name from metadata
	enrichmentFiles := make(map[string]*SimpleEnrichmentData)
	for _, dir := range modelDirs {
		enrichmentFile := filepath.Join(dir, "models", "enrichment.yaml")
		metadataFile := filepath.Join(dir, "models", "metadata.yaml")

		if _, err := os.Stat(enrichmentFile); err == nil {
			// Read the enrichment data
			data, err := os.ReadFile(enrichmentFile)
			if err != nil {
				continue
			}

			var enriched SimpleEnrichmentData
			if err := yaml.Unmarshal(data, &enriched); err != nil {
				continue
			}

			// Try to read the corresponding metadata to find the actual model name
			if metadataData, err := os.ReadFile(metadataFile); err == nil {
				var metadata types.ExtractedMetadata
				if err := yaml.Unmarshal(metadataData, &metadata); err == nil && metadata.Name != nil {
					enrichmentFiles[*metadata.Name] = &enriched
				}
			}
		}
	}

	// Match models with their enrichment data
	for _, model := range models {
		modelName := ""
		if model.Name != nil {
			modelName = *model.Name
		}

		if enriched, exists := enrichmentFiles[modelName]; exists {
			enrichmentData[modelName] = enriched
		}
	}

	return enrichmentData, nil
}

// generateReport creates the metadata report
func generateReport(catalog *types.ModelsCatalog, enrichmentData map[string]*SimpleEnrichmentData) *MetadataReport {
	report := &MetadataReport{
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			TotalModels:       len(catalog.Models),
			FieldCompleteness: make(map[string]Completeness),
			DataSources:       make(map[string]int),
		},
		Models: make([]ModelReport, 0, len(catalog.Models)),
	}

	// Field names we want to track
	trackedFields := []string{
		"name", "provider", "description", "readme", "language", "license",
		"licenseLink", "tasks", "artifacts",
		"createTimeSinceEpoch",
	}

	// Initialize field completeness tracking
	for _, field := range trackedFields {
		report.Summary.FieldCompleteness[field] = Completeness{}
	}

	// Analyze each model
	for _, model := range catalog.Models {
		modelName := ""
		if model.Name != nil {
			modelName = *model.Name
		}
		modelReport := analyzeModel(model, enrichmentData[modelName], trackedFields)
		report.Models = append(report.Models, modelReport)

		// Update summary statistics
		updateSummaryStats(&report.Summary, modelReport, trackedFields)
	}

	// Calculate percentages
	for field, comp := range report.Summary.FieldCompleteness {
		total := comp.Populated + comp.Null
		if total > 0 {
			comp.Percentage = float64(comp.Populated) / float64(total) * 100
			report.Summary.FieldCompleteness[field] = comp
		}
	}

	return report
}

// analyzeModel analyzes a single model's metadata completeness and sources
func analyzeModel(model types.CatalogMetadata, enriched *SimpleEnrichmentData, trackedFields []string) ModelReport {
	modelName := ""
	if model.Name != nil {
		modelName = *model.Name
	}

	modelProvider := ""
	if model.Provider != nil {
		modelProvider = *model.Provider
	}

	modelReport := ModelReport{
		Name:            modelName,
		Provider:        modelProvider,
		Fields:          make(map[string]FieldStatus),
		DataSources:     make(map[string]int),
		SourceBreakdown: SourceBreakdown{},
	}

	// Analyze each tracked field
	for _, fieldName := range trackedFields {
		status := analyzeField(fieldName, model, enriched)
		modelReport.Fields[fieldName] = status

		if status.IsNull {
			modelReport.MissingFields = append(modelReport.MissingFields, fieldName)
		} else {
			modelReport.DataSources[status.Source]++
			// Update source breakdown with granular tracking
			updateSourceBreakdown(&modelReport.SourceBreakdown, status.Source)
		}
	}

	return modelReport
}

// updateSourceBreakdown updates the source breakdown with granular tracking
func updateSourceBreakdown(breakdown *SourceBreakdown, source string) {
	switch source {
	case "modelcard.yaml":
		breakdown.ModelcardYAML++
	case "modelcard.regex", "modelcard.inferred":
		breakdown.ModelcardRegex++
	case "huggingface.yaml":
		breakdown.HuggingfaceYAML++
	case "huggingface.tags":
		breakdown.HuggingfaceTags++
	case "huggingface.regex", "huggingface.api":
		breakdown.HuggingfaceRegex++
	case "registry":
		breakdown.Registry++
	case "generated":
		breakdown.Generated++
	default:
		breakdown.Other++
	}
}

// getDetectionMethod extracts the detection method from a source string
func getDetectionMethod(source string) string {
	switch {
	case strings.HasSuffix(source, ".yaml"):
		return "YAML frontmatter"
	case strings.HasSuffix(source, ".regex"):
		return "Regex extraction"
	case strings.HasSuffix(source, ".api"):
		return "API call"
	case strings.HasSuffix(source, ".tags"):
		return "Tags metadata"
	case source == "generated":
		return "Generated"
	case source == "registry":
		return "Registry artifacts"
	default:
		return "Unknown"
	}
}

// analyzeField analyzes a specific field for a model
func analyzeField(fieldName string, model types.CatalogMetadata, enriched *SimpleEnrichmentData) FieldStatus {
	status := FieldStatus{
		Source:          "unknown",
		DetectionMethod: "Unknown",
		IsNull:          true,
	}

	switch fieldName {
	case "name":
		if model.Name != nil && *model.Name != "" {
			status.Value = *model.Name
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "name")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "provider":
		if model.Provider != nil && *model.Provider != "" {
			status.Value = *model.Provider
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "provider")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "description":
		if model.Description != nil && *model.Description != "" {
			status.Value = *model.Description
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "description")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "readme":
		if model.Readme != nil && *model.Readme != "" {
			status.Value = "present"
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "readme")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "language":
		if len(model.Language) > 0 {
			status.Value = model.Language
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "language")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "license":
		if model.License != nil && *model.License != "" {
			status.Value = *model.License
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "license")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "licenseLink":
		if model.LicenseLink != nil && *model.LicenseLink != "" {
			status.Value = *model.LicenseLink
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "licenseLink")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "tasks":
		if len(model.Tasks) > 0 {
			status.Value = model.Tasks
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "tasks")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	case "artifacts":
		if len(model.Artifacts) > 0 {
			status.Value = len(model.Artifacts)
			status.IsNull = false
			status.Source = "registry" // Artifacts typically come from OCI registry
			status.DetectionMethod = "Registry artifacts"
		}
	case "createTimeSinceEpoch":
		if model.CreateTimeSinceEpoch != nil && *model.CreateTimeSinceEpoch != "" {
			status.Value = *model.CreateTimeSinceEpoch
			status.IsNull = false
			status.Source = getSourceFromEnriched(enriched, "createTimeSinceEpoch")
			status.DetectionMethod = getDetectionMethod(status.Source)
		}
	}

	// Check if the value is empty even if not null
	if !status.IsNull {
		switch v := status.Value.(type) {
		case string:
			status.IsEmpty = v == ""
		case []string:
			status.IsEmpty = len(v) == 0
		case []types.OCIArtifact:
			status.IsEmpty = len(v) == 0
		}
	}

	return status
}

// getSourceFromEnriched extracts the data source from enriched metadata
func getSourceFromEnriched(enriched *SimpleEnrichmentData, fieldName string) string {
	if enriched == nil || enriched.DataSources == nil {
		return "modelcard.regex"
	}

	// Map field names to their keys in data_sources
	var sourceKey string
	switch fieldName {
	case "name":
		sourceKey = "name"
	case "provider":
		sourceKey = "provider"
	case "description":
		sourceKey = "description"
	case "license":
		sourceKey = "license"
	case "tasks":
		sourceKey = "tasks"
	case "createTimeSinceEpoch":
		sourceKey = "create_time_since_epoch"
	case "readme":
		sourceKey = "readme"
	case "language":
		sourceKey = "language"
	case "licenseLink":
		sourceKey = "license_link"
	default:
		return "modelcard.regex"
	}

	if source, exists := enriched.DataSources[sourceKey]; exists && source != "" {
		return source
	}

	return "modelcard.regex"
}

// updateSummaryStats updates the summary statistics
func updateSummaryStats(summary *ReportSummary, modelReport ModelReport, trackedFields []string) {
	for _, field := range trackedFields {
		status := modelReport.Fields[field]
		comp := summary.FieldCompleteness[field]

		if status.IsNull {
			comp.Null++
		} else {
			comp.Populated++
			summary.DataSources[status.Source]++
		}

		summary.FieldCompleteness[field] = comp
	}
}

// writeMarkdownReport writes the report in markdown format
func writeMarkdownReport(report *MetadataReport, outputPath string) error {
	var md strings.Builder

	// Title and generation info
	md.WriteString("# Model Metadata Completeness Report\n\n")
	fmt.Fprintf(&md, "**Generated:** %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05 UTC"))

	// Summary section
	md.WriteString("## Summary\n\n")
	fmt.Fprintf(&md, "**Total Models:** %d\n\n", report.Summary.TotalModels)

	// Field completeness table
	md.WriteString("### Field Completeness\n\n")
	md.WriteString("| Field | Populated | Null | Percentage |\n")
	md.WriteString("|-------|-----------|------|------------|\n")

	// Sort fields by completion percentage
	type fieldComp struct {
		name string
		comp Completeness
	}
	var sortedFields []fieldComp
	for field, comp := range report.Summary.FieldCompleteness {
		sortedFields = append(sortedFields, fieldComp{field, comp})
	}
	sort.Slice(sortedFields, func(i, j int) bool {
		return sortedFields[i].comp.Percentage > sortedFields[j].comp.Percentage
	})

	for _, fc := range sortedFields {
		fmt.Fprintf(&md, "| %s | %d | %d | %.1f%% |\n",
			fc.name, fc.comp.Populated, fc.comp.Null, fc.comp.Percentage)
	}

	// Data sources summary
	md.WriteString("\n### Data Sources\n\n")
	md.WriteString("| Source | Count | Percentage |\n")
	md.WriteString("|--------|-------|------------|\n")

	total := 0
	for _, count := range report.Summary.DataSources {
		total += count
	}

	type sourceCount struct {
		source string
		count  int
	}
	var sortedSources []sourceCount
	for source, count := range report.Summary.DataSources {
		sortedSources = append(sortedSources, sourceCount{source, count})
	}
	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].count > sortedSources[j].count
	})

	for _, sc := range sortedSources {
		percentage := float64(sc.count) / float64(total) * 100
		fmt.Fprintf(&md, "| %s | %d | %.1f%% |\n", sc.source, sc.count, percentage)
	}

	// Source breakdown summary
	md.WriteString("\n### Detailed Source Breakdown\n\n")
	md.WriteString("| Source Type | Count | Percentage |\n")
	md.WriteString("|-------------|-------|------------|\n")

	sourceBreakdownSummary := make(map[string]int)
	for _, model := range report.Models {
		sourceBreakdownSummary["Modelcard YAML"] += model.SourceBreakdown.ModelcardYAML
		sourceBreakdownSummary["Modelcard Regex"] += model.SourceBreakdown.ModelcardRegex
		sourceBreakdownSummary["HuggingFace YAML"] += model.SourceBreakdown.HuggingfaceYAML
		sourceBreakdownSummary["HuggingFace Tags"] += model.SourceBreakdown.HuggingfaceTags
		sourceBreakdownSummary["HuggingFace Regex"] += model.SourceBreakdown.HuggingfaceRegex
		sourceBreakdownSummary["Registry"] += model.SourceBreakdown.Registry
		sourceBreakdownSummary["Generated"] += model.SourceBreakdown.Generated
		sourceBreakdownSummary["Other"] += model.SourceBreakdown.Other
	}

	// Sort by count descending
	type sourceBreakdownEntry struct {
		name  string
		count int
	}
	var sortedBreakdown []sourceBreakdownEntry
	for name, count := range sourceBreakdownSummary {
		if count > 0 {
			sortedBreakdown = append(sortedBreakdown, sourceBreakdownEntry{name, count})
		}
	}
	sort.Slice(sortedBreakdown, func(i, j int) bool {
		return sortedBreakdown[i].count > sortedBreakdown[j].count
	})

	for _, entry := range sortedBreakdown {
		percentage := float64(entry.count) / float64(total) * 100
		fmt.Fprintf(&md, "| %s | %d | %.1f%% |\n", entry.name, entry.count, percentage)
	}

	// Individual model reports
	md.WriteString("\n## Individual Model Reports\n\n")

	for _, model := range report.Models {
		fmt.Fprintf(&md, "### %s\n\n", model.Name)

		if model.Provider != "" {
			fmt.Fprintf(&md, "**Provider:** %s\n\n", model.Provider)
		}

		// Missing fields
		if len(model.MissingFields) > 0 {
			md.WriteString("**Missing Fields:** ")
			md.WriteString(strings.Join(model.MissingFields, ", "))
			md.WriteString("\n\n")
		}

		// Source breakdown for this model
		yamlFields := model.SourceBreakdown.ModelcardYAML + model.SourceBreakdown.HuggingfaceYAML
		totalFields := yamlFields + model.SourceBreakdown.ModelcardRegex + model.SourceBreakdown.HuggingfaceTags +
			model.SourceBreakdown.HuggingfaceRegex + model.SourceBreakdown.Registry +
			model.SourceBreakdown.Generated + model.SourceBreakdown.Other

		if totalFields > 0 {
			yamlPercentage := float64(yamlFields) / float64(totalFields) * 100
			fmt.Fprintf(&md, "**YAML Frontmatter Health:** %.1f%% (%d/%d fields from YAML)\n\n", yamlPercentage, yamlFields, totalFields)
		}

		// Field details
		md.WriteString("| Field | Value | Source | Detection Method | Status |\n")
		md.WriteString("|-------|-------|--------|------------------|--------|\n")

		// Sort fields alphabetically
		var fieldNames []string
		for field := range model.Fields {
			fieldNames = append(fieldNames, field)
		}
		sort.Strings(fieldNames)

		for _, field := range fieldNames {
			status := model.Fields[field]
			valueStr := formatValue(status.Value)
			statusStr := "✅"
			if status.IsNull {
				statusStr = "❌ null"
				valueStr = "—"
			} else if status.IsEmpty {
				statusStr = "⚠️ empty"
			}

			fmt.Fprintf(&md, "| %s | %s | %s | %s | %s |\n",
				field, valueStr, status.Source, status.DetectionMethod, statusStr)
		}

		md.WriteString("\n")
	}

	return os.WriteFile(outputPath, []byte(md.String()), 0644)
}

// writeYAMLReport writes the report in YAML format
func writeYAMLReport(report *MetadataReport, outputPath string) error {
	data, err := yaml.Marshal(report)
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// formatValue formats a value for display in the markdown table
func formatValue(value interface{}) string {
	if value == nil {
		return "—"
	}

	switch v := value.(type) {
	case string:
		if len(v) > 50 {
			return fmt.Sprintf("%.50s...", v)
		}
		return v
	case []string:
		if len(v) == 0 {
			return "—"
		}
		if len(v) == 1 {
			return v[0]
		}
		return fmt.Sprintf("%s (+%d more)", v[0], len(v)-1)
	case int, int64:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
