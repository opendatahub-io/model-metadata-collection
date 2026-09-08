package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/opendatahub-io/model-metadata-collection/internal/report"
)

func main() {
	var (
		catalogPath = flag.String("catalog", "data/validated-models-catalog.yaml", "Path to the models catalog YAML file")
		outputDir   = flag.String("output-dir", "output", "Directory containing model extraction output")
		reportDir   = flag.String("report-dir", "", "Directory to write reports (defaults to output-dir)")
		help        = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		printUsage()
		os.Exit(0)
	}

	// Validate inputs
	if err := validateInputs(*catalogPath, *outputDir); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Set default report directory
	if *reportDir == "" {
		*reportDir = *outputDir
	}

	// Ensure report directory exists
	if err := os.MkdirAll(*reportDir, 0755); err != nil {
		log.Fatalf("Failed to create report directory: %v", err)
	}

	// Generate the report
	fmt.Printf("Generating metadata completeness report...\n")
	fmt.Printf("  Catalog: %s\n", *catalogPath)
	fmt.Printf("  Output dir: %s\n", *outputDir)
	fmt.Printf("  Report dir: %s\n", *reportDir)
	fmt.Println()

	if err := report.GenerateMetadataReport(*catalogPath, *outputDir, *reportDir); err != nil {
		log.Fatalf("Failed to generate report: %v", err)
	}

	fmt.Println("✅ Metadata report generation completed successfully!")
}

func printUsage() {
	fmt.Println("Metadata Report Generator")
	fmt.Println()
	fmt.Println("Generates comprehensive reports about metadata completeness and data sources")
	fmt.Println("for models processed by the model-extractor tool.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  metadata-report [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Generate report with defaults")
	fmt.Println("  metadata-report")
	fmt.Println()
	fmt.Println("  # Custom paths")
	fmt.Println("  metadata-report -catalog=my-catalog.yaml -output-dir=my-output")
	fmt.Println()
	fmt.Println("  # Write reports to specific directory")
	fmt.Println("  metadata-report -report-dir=reports")
	fmt.Println()
	fmt.Println("Output:")
	fmt.Println("  - metadata-report.md  (Human-readable markdown report)")
	fmt.Println("  - metadata-report.yaml (Machine-readable detailed data)")
}

func validateInputs(catalogPath, outputDir string) error {
	// Check if catalog file exists
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		return fmt.Errorf("catalog file does not exist: %s", catalogPath)
	}

	// Check if output directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("output directory does not exist: %s", outputDir)
	}

	// Check if output directory contains model data
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("cannot read output directory: %v", err)
	}

	hasModelData := false
	for _, entry := range entries {
		if entry.IsDir() {
			modelsDir := filepath.Join(outputDir, entry.Name(), "models")
			if _, err := os.Stat(modelsDir); err == nil {
				hasModelData = true
				break
			}
		}
	}

	if !hasModelData {
		return fmt.Errorf("output directory does not contain model extraction data: %s", outputDir)
	}

	return nil
}
