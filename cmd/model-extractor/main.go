package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/containers/image/v5/docker"
	blobinfocachememory "github.com/containers/image/v5/pkg/blobinfocache/memory"
	containertypes "github.com/containers/image/v5/types"
	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
	"github.com/opendatahub-io/model-metadata-collection/internal/config"
	"github.com/opendatahub-io/model-metadata-collection/internal/enrichment"
	"github.com/opendatahub-io/model-metadata-collection/internal/huggingface"
	"github.com/opendatahub-io/model-metadata-collection/internal/metadata"
	"github.com/opendatahub-io/model-metadata-collection/internal/registry"
	"github.com/opendatahub-io/model-metadata-collection/pkg/types"
	"github.com/opendatahub-io/model-metadata-collection/pkg/utils"
)

// Command line flags
var (
	modelsIndexPath          = flag.String("input", "data/models-index.yaml", "Path to models index YAML file")
	outputDir                = flag.String("output-dir", "output", "Output directory for extracted metadata")
	catalogOutputPath        = flag.String("catalog-output", "data/models-catalog.yaml", "Path for the generated models catalog")
	maxConcurrent            = flag.Int("max-concurrent", 5, "Maximum number of concurrent model processing jobs")
	skipHuggingFace          = flag.Bool("skip-huggingface", false, "Skip HuggingFace collection processing and enrichment")
	skipEnrichment           = flag.Bool("skip-enrichment", false, "Skip metadata enrichment from HuggingFace")
	skipCatalog              = flag.Bool("skip-catalog", false, "Skip catalog generation")
	staticCatalogFiles       = flag.String("static-catalog-files", "", "Comma-separated list of static catalog files to include")
	skipDefaultStaticCatalog = flag.Bool("skip-default-static-catalog", false, "Skip processing the default input/supplemental-catalog.yaml file")
	help                     = flag.Bool("help", false, "Show help message")
)

// ModelResult represents the result of processing a single model
type ModelResult struct {
	Ref            string
	ModelCardFound bool
	Metadata       types.ModelMetadata
}

func main() {
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	log.Printf("Starting model metadata collection with configuration:")
	log.Printf("  Models Index: %s", *modelsIndexPath)
	log.Printf("  Output Directory: %s", *outputDir)
	log.Printf("  Catalog Output: %s", *catalogOutputPath)
	log.Printf("  Max Concurrent: %d", *maxConcurrent)
	log.Printf("  Skip HuggingFace: %v", *skipHuggingFace)
	log.Printf("  Skip Enrichment: %v", *skipEnrichment)
	log.Printf("  Skip Catalog: %v", *skipCatalog)
	log.Printf("  Static Catalog Files: %s", *staticCatalogFiles)
	log.Printf("  Skip Default Static Catalog: %v", *skipDefaultStaticCatalog)

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Ensure catalog output directory exists
	catalogDir := filepath.Dir(*catalogOutputPath)
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		log.Fatalf("Failed to create catalog output directory: %v", err)
	}

	// Process HuggingFace collections (unless skipped)
	if !*skipHuggingFace {
		log.Println("Processing HuggingFace collections...")
		err := huggingface.ProcessCollections()
		if err != nil {
			log.Printf("Warning: Failed to process HuggingFace collections: %v", err)
			log.Println("Falling back to existing models-index.yaml")
		}
	}

	// Load models from configuration file
	modelEntries, err := loadModelsWithMetadata(*modelsIndexPath)
	if err != nil {
		log.Fatalf("Failed to load models: %v", err)
	}

	log.Printf("Processing %d models...", len(modelEntries))

	// Process models in parallel
	modelResults := processModelsInParallelWithMetadata(modelEntries, *maxConcurrent)

	// Generate manifests.yaml
	err = generateManifestsYAML(modelResults, *outputDir)
	if err != nil {
		log.Fatalf("Failed to generate manifests.yaml: %v", err)
	}

	log.Printf("All manifest processing completed")

	// Enrich registry model metadata with HuggingFace data (unless skipped)
	// This happens AFTER model processing to enrich the extracted metadata
	if !*skipEnrichment {
		log.Println("Enriching extracted metadata with HuggingFace data...")
		err := enrichment.EnrichMetadataFromHuggingFace()
		if err != nil {
			log.Printf("Warning: Failed to enrich metadata: %v", err)
		}

		// Update all existing models with OCI artifact metadata
		err = enrichment.UpdateAllModelsWithOCIArtifacts()
		if err != nil {
			log.Printf("Warning: Failed to update OCI artifacts: %v", err)
		}
	}

	// Create the models catalog (unless skipped)
	if !*skipCatalog {
		// Load static catalogs
		staticCatalogPaths := getStaticCatalogPaths(*staticCatalogFiles, *skipDefaultStaticCatalog)

		var staticModels []types.CatalogMetadata
		if len(staticCatalogPaths) > 0 {
			log.Printf("Loading static catalogs...")
			loadedStaticModels, err := catalog.LoadStaticCatalogs(staticCatalogPaths)
			if err != nil {
				log.Printf("Warning: Failed to load static catalogs: %v", err)
				staticModels = []types.CatalogMetadata{} // Continue with empty static models
			} else {
				staticModels = loadedStaticModels
			}
		} else {
			log.Printf("No static catalog files to process")
			staticModels = []types.CatalogMetadata{}
		}

		// Create the models catalog with both dynamic and static models
		log.Printf("Creating models catalog...")
		err = catalog.CreateModelsCatalogWithStatic(*outputDir, *catalogOutputPath, staticModels)
		if err != nil {
			log.Fatalf("Failed to create models catalog: %v", err)
		}
	}

	log.Println("Model metadata collection completed successfully!")
}

// printHelp displays comprehensive usage information and command-line examples.
// It provides users with detailed guidance on how to use the model metadata
// collection tool effectively with various configuration options.
func printHelp() {
	fmt.Println("Model Metadata Collection Tool")
	fmt.Println("")
	fmt.Println("This tool extracts metadata from Red Hat AI model containers and enriches it with HuggingFace data.")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  # Basic usage with default settings")
	fmt.Printf("  %s\n", os.Args[0])
	fmt.Println("")
	fmt.Println("  # Custom input and output paths")
	fmt.Printf("  %s --input custom-models.yaml --output-dir /tmp/output --catalog-output /tmp/catalog.yaml\n", os.Args[0])
	fmt.Println("")
	fmt.Println("  # Skip HuggingFace processing and enrichment")
	fmt.Printf("  %s --skip-huggingface --skip-enrichment\n", os.Args[0])
	fmt.Println("")
	fmt.Println("  # Process only metadata extraction")
	fmt.Printf("  %s --skip-huggingface --skip-enrichment --skip-catalog\n", os.Args[0])
	fmt.Println("")
	fmt.Println("  # Include custom static catalog files")
	fmt.Printf("  %s --static-catalog-files custom1.yaml,custom2.yaml\n", os.Args[0])
	fmt.Println("")
	fmt.Println("  # Skip default static catalog but include custom ones")
	fmt.Printf("  %s --skip-default-static-catalog --static-catalog-files custom.yaml\n", os.Args[0])
}

// getStaticCatalogPaths constructs the complete list of static catalog files to process
// based on CLI flags. It handles both custom catalog files and the default supplemental
// catalog, providing flexible catalog management.
//
// Parameters:
//   - staticCatalogFiles: comma-separated string of custom catalog file paths
//   - skipDefaultStaticCatalog: whether to exclude the default supplemental catalog
//
// Returns:
//   - []string: slice of file paths to process, may be empty if all sources skipped
func getStaticCatalogPaths(staticCatalogFiles string, skipDefaultStaticCatalog bool) []string {
	var paths []string

	// Add custom static catalog files if specified
	if staticCatalogFiles != "" {
		customPaths := strings.Split(staticCatalogFiles, ",")
		for _, path := range customPaths {
			path = strings.TrimSpace(path)
			if path != "" {
				paths = append(paths, path)
			}
		}
	}

	// Add default static catalog file if not skipped and exists
	if !skipDefaultStaticCatalog {
		defaultPath := "input/supplemental-catalog.yaml"
		if _, err := os.Stat(defaultPath); err == nil {
			paths = append(paths, defaultPath)
		}
	}

	return paths
}

// loadModelsWithMetadata loads model configurations from the specified index file with
// intelligent fallback to version-specific index files. It provides resilient model
// loading that adapts to different index file formats and availability.
//
// Parameters:
//   - modelsIndexPath: primary path to models index YAML file
//
// Returns:
//   - []types.ModelEntry: slice of model entries with metadata
//   - error: loading or parsing errors, nil on success
func loadModelsWithMetadata(modelsIndexPath string) ([]types.ModelEntry, error) {
	// First try to load from specified models index file
	if _, err := os.Stat(modelsIndexPath); err == nil {
		log.Printf("Loading models from: %s", modelsIndexPath)
		return config.LoadModelsConfigFromYAML(modelsIndexPath)
	}

	// Try to load from latest version index file as fallback
	latestIndexFile, err := getLatestVersionIndexFile()
	if err == nil {
		log.Printf("Using latest version index file: %s", latestIndexFile)
		// Convert version index to model entries (all validated=true, featured=false by default)
		modelURIs, err := config.LoadModelsFromVersionIndex(latestIndexFile)
		if err != nil {
			return nil, err
		}

		var modelEntries []types.ModelEntry
		for _, uri := range modelURIs {
			modelEntries = append(modelEntries, types.ModelEntry{
				Type:   "oci",
				URI:    uri,
				Labels: []string{"validated"},
			})
		}
		return modelEntries, nil
	}

	return nil, fmt.Errorf("no valid models index file found at %s and no version index files available", modelsIndexPath)
}

// getLatestVersionIndexFile discovers and returns the most recent version-specific
// index file from the data directory. It uses glob patterns to find HuggingFace
// collection index files and returns the lexicographically latest version.
//
// Returns:
//   - string: path to the latest version index file
//   - error: filesystem or discovery errors, nil on success
func getLatestVersionIndexFile() (string, error) {
	files, err := filepath.Glob("data/hugging-face-redhat-ai-validated-v*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to find version index files: %v", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no version index files found")
	}

	sort.Strings(files) // explicit lexicographic order
	return files[len(files)-1], nil
}

// processModelsInParallelWithMetadata orchestrates concurrent processing of multiple
// model entries, extracting metadata from their container images. It converts model
// entries to manifest references and delegates to the core parallel processing logic.
//
// Parameters:
//   - modelEntries: slice of model entries containing URIs and metadata
//   - maxConcurrent: maximum number of concurrent processing goroutines
//
// Returns:
//   - []ModelResult: slice of processing results for each model
func processModelsInParallelWithMetadata(modelEntries []types.ModelEntry, maxConcurrent int) []ModelResult {
	// Extract URIs for processing
	var manifestRefs []string
	uriToEntry := make(map[string]types.ModelEntry)

	for _, entry := range modelEntries {
		manifestRefs = append(manifestRefs, entry.URI)
		uriToEntry[entry.URI] = entry
	}

	return processModelsInParallelWithEntryMap(manifestRefs, uriToEntry, maxConcurrent)
}

// processModelsInParallelWithEntryMap performs the core parallel processing of model
// container images with concurrency control. It uses goroutines with semaphore-based
// limiting to extract modelcard metadata while respecting system resource constraints.
//
// Parameters:
//   - manifestRefs: slice of container manifest references to process
//   - uriToEntry: mapping from URI to ModelEntry for metadata lookup
//   - maxConcurrent: maximum number of concurrent processing goroutines
//
// Returns:
//   - []ModelResult: slice of processing results with metadata extraction status
func processModelsInParallelWithEntryMap(manifestRefs []string, uriToEntry map[string]types.ModelEntry, maxConcurrent int) []ModelResult {
	sys := &containertypes.SystemContext{}

	// Create a WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Create a semaphore to limit concurrent goroutines
	if maxConcurrent < 1 {
		log.Printf("Warning: invalid --max-concurrent=%d; defaulting to 1", maxConcurrent)
		maxConcurrent = 1
	}
	semaphore := make(chan struct{}, maxConcurrent)

	// Channel to collect results from goroutines
	results := make(chan ModelResult, len(manifestRefs))

	// Process each manifest reference in parallel with concurrency limit
	for _, manifestRef := range manifestRefs {
		// Acquire semaphore (blocks if max goroutines are already running)
		semaphore <- struct{}{}

		wg.Add(1)
		go func(ref string, entry types.ModelEntry) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done

			log.Printf("Starting processing for: %s", ref)
			src, layers, configBlob, err := fetchManifestSrcAndLayers(ref, sys)
			if err != nil {
				log.Printf("Warning: Failed to fetch manifest for %s: %v", ref, err)
				// Send failed result to channel
				results <- ModelResult{
					Ref:            ref,
					ModelCardFound: false,
					Metadata:       types.ModelMetadata{},
				}
				return
			}
			defer func() { _ = src.Close() }()
			modelCardFound, metadata := scanLayersForModelCardWithTags(layers, src, ref, configBlob, entry)
			log.Printf("Completed processing for: %s", ref)

			// Send result to channel
			results <- ModelResult{
				Ref:            ref,
				ModelCardFound: modelCardFound,
				Metadata:       metadata,
			}
		}(manifestRef, uriToEntry[manifestRef])
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect all results
	var modelResults []ModelResult
	for result := range results {
		modelResults = append(modelResults, result)
	}

	return modelResults
}

// scanLayersForModelCardWithTags scans container layers for model card content and adds model labels as tags
func scanLayersForModelCardWithTags(layers []containertypes.BlobInfo, src containertypes.ImageSource, manifestRef string, configBlob []byte, entry types.ModelEntry) (bool, types.ModelMetadata) {
	modelCardFound, metadata := scanLayersForModelCard(layers, src, manifestRef, configBlob)

	// Add labels from the model entry as tags to the extracted metadata
	// This works for both successful extractions and skeleton metadata
	addModelLabelTags(manifestRef, entry)

	return modelCardFound, metadata
}

// addModelLabelTags adds model labels as tags to the extracted metadata
func addModelLabelTags(manifestRef string, entry types.ModelEntry) {
	// Create sanitized directory name for the model
	sanitizedName := utils.SanitizeManifestRef(manifestRef)
	metadataPath := filepath.Join(*outputDir, sanitizedName, "models", "metadata.yaml")

	// Read existing metadata
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		log.Printf("Warning: Could not read metadata file %s: %v", metadataPath, err)
		return
	}

	// Parse existing metadata
	var metadata types.ExtractedMetadata
	err = yaml.Unmarshal(data, &metadata)
	if err != nil {
		log.Printf("Warning: Could not parse metadata file %s: %v", metadataPath, err)
		return
	}

	// Initialize tags slice if nil
	if metadata.Tags == nil {
		metadata.Tags = []string{}
	}

	// Track if we made changes
	changed := false

	// Add each label from the model entry as a tag if not already present
	for _, label := range entry.Labels {
		if label != "" && !contains(metadata.Tags, label) {
			metadata.Tags = append(metadata.Tags, label)
			changed = true
			log.Printf("Added '%s' tag to %s", label, manifestRef)
		}
	}

	// Write back the metadata if changes were made
	if changed {
		updatedData, err := yaml.Marshal(&metadata)
		if err != nil {
			log.Printf("Warning: Could not marshal updated metadata for %s: %v", manifestRef, err)
			return
		}

		err = os.WriteFile(metadataPath, updatedData, 0644)
		if err != nil {
			log.Printf("Warning: Could not write updated metadata file %s: %v", metadataPath, err)
			return
		}
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// scanLayersForModelCard scans container layers for model card content
func scanLayersForModelCard(layers []containertypes.BlobInfo, src containertypes.ImageSource, manifestRef string, configBlob []byte) (bool, types.ModelMetadata) {
	for i, layer := range layers {
		log.Printf("Layer %d:", i+1)
		log.Printf("  Digest: %s", layer.Digest)
		log.Printf("  MediaType: %s", layer.MediaType)
		log.Printf("  Size: %d bytes", layer.Size)
		if layer.Annotations != nil {
			log.Printf("  Annotations: %v", layer.Annotations)

			// Check if this layer has the modelcard annotation
			if layerType, exists := layer.Annotations["io.opendatahub.modelcar.layer.type"]; exists && layerType == "modelcard" {
				log.Printf("  Found modelcard layer! Attempting to access modelcard layer blob with digest: %s", layer.Digest)

				var layerBlob io.ReadCloser
				var err error

				ctxBlob, cancelBlob := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancelBlob()
				layerBlob, _, err = src.GetBlob(ctxBlob, containertypes.BlobInfo{
					Digest: layer.Digest,
				}, blobinfocachememory.New())
				if err != nil {
					log.Printf("Warning: Failed to get modelcard layer blob for %s: %v", manifestRef, err)
					continue
				}

				if layerBlob == nil {
					log.Printf("layerBlob is nil for modelcard layer")
				} else {
					var reader io.Reader = layerBlob
					defer func() { _ = layerBlob.Close() }()
					log.Printf("  Successfully fetched modelcard layer blob. Attempting to read as tar...")

					// Check if it's a gzipped tar file
					if strings.Contains(layer.MediaType, "+gzip") {
						log.Printf("  Detected gzipped tar file, decompressing...")
						gzReader, err := gzip.NewReader(layerBlob)
						if err != nil {
							log.Printf("Error creating gzip reader: %v", err)
							continue
						}
						defer func() { _ = gzReader.Close() }()
						reader = gzReader
					}

					tr := tar.NewReader(reader)
					var mdFileCount int
					var singleMdFileName string
					var singleMdContent []byte

					for {
						header, err := tr.Next()
						if err == io.EOF {
							break
						}
						if err != nil {
							log.Printf("Error reading tar: %v", err)
							break
						}
						log.Printf("  Found file in tar: %s (size: %d bytes)", header.Name, header.Size)
						if strings.HasSuffix(header.Name, ".md") {
							mdFileCount++
							if mdFileCount > 1 {
								log.Printf("  Found multiple .md files, skipping content display")
								break
							}
							singleMdFileName = header.Name
							// Only read content if this is the first (and potentially only) .md file
							var content bytes.Buffer
							_, err := io.Copy(&content, tr)
							if err != nil {
								log.Printf("Error reading %s: %v", header.Name, err)
								continue
							}
							singleMdContent = content.Bytes()
						} else {
							// Skip non-.md files
							_, err := io.Copy(io.Discard, tr)
							if err != nil {
								log.Printf("Error skipping %s: %v", header.Name, err)
								continue
							}
						}
					}

					if mdFileCount == 1 {
						log.Printf("  Found single .md file: %s (size: %d bytes)", singleMdFileName, len(singleMdContent))

						// Create output directory
						sanitizedDir := utils.SanitizeManifestRef(manifestRef)
						modelDir := filepath.Join(*outputDir, sanitizedDir)

						// Sanitize tar entry path to prevent path traversal
						safeName := filepath.Clean(singleMdFileName)
						if filepath.IsAbs(safeName) || strings.HasPrefix(safeName, ".."+string(os.PathSeparator)) {
							log.Printf("Warning: Skipping unsafe tar entry path: %s", singleMdFileName)
							continue
						}
						outputFilePath := filepath.Join(modelDir, safeName)
						// Ensure final path remains within modelDir
						modelDirClean := filepath.Clean(modelDir) + string(os.PathSeparator)
						outputFilePathClean := filepath.Clean(outputFilePath)
						if !strings.HasPrefix(outputFilePathClean, modelDirClean) && outputFilePathClean != filepath.Clean(modelDir) {
							log.Printf("Warning: Skipping potential path traversal: %s", singleMdFileName)
							continue
						}
						outputFileDir := filepath.Dir(outputFilePath)
						err := os.MkdirAll(outputFileDir, 0755)
						if err != nil {
							log.Printf("Warning: Failed to create output directory for %s: %v", manifestRef, err)
							continue
						}

						// Write modelcard content to file
						err = os.WriteFile(outputFilePath, singleMdContent, 0644)
						if err != nil {
							log.Printf("Warning: Failed to write modelcard content to file for %s: %v", manifestRef, err)
							continue
						}

						log.Printf("  Successfully wrote modelcard content to: %s", outputFilePath)

						// Parse metadata from the modelcard content
						metadataFlags := metadata.ParseModelCardMetadata(singleMdContent)

						// Extract actual metadata values
						extractedMetadata := metadata.ExtractMetadataValues(singleMdContent)

						// Populate artifacts with OCI registry metadata and real timestamps
						extractedMetadata.Artifacts = registry.ExtractOCIArtifactsFromRegistry(manifestRef)

						// Extract real timestamps from config blob and update artifacts
						createTime, updateTime := extractTimestampsFromConfig(configBlob)
						for i := range extractedMetadata.Artifacts {
							if extractedMetadata.Artifacts[i].CreateTimeSinceEpoch == nil {
								extractedMetadata.Artifacts[i].CreateTimeSinceEpoch = createTime
							}
							if extractedMetadata.Artifacts[i].LastUpdateTimeSinceEpoch == nil {
								extractedMetadata.Artifacts[i].LastUpdateTimeSinceEpoch = updateTime
							}
						}

						// Generate metadata.yaml file in the same directory
						metadataFilePath := filepath.Join(outputFileDir, "metadata.yaml")
						metadataYaml, err := yaml.Marshal(&extractedMetadata)
						if err != nil {
							log.Printf("Failed to marshal metadata to YAML: %v", err)
						} else {
							err = os.WriteFile(metadataFilePath, metadataYaml, 0644)
							if err != nil {
								log.Printf("Failed to write metadata.yaml: %v", err)
							} else {
								log.Printf("  Successfully wrote metadata.yaml to: %s", metadataFilePath)
							}
						}

						return true, metadataFlags
					} else {
						log.Printf("  No .md files found in the blob")
					}
				}
			}
		}
	}

	// If no modelcard was found, create a skeleton metadata.yaml for enrichment processing
	log.Printf("  No modelcard layer found, creating skeleton metadata for enrichment")
	createSkeletonMetadata(manifestRef, configBlob)

	return false, types.ModelMetadata{}
}

// createSkeletonMetadata generates fallback metadata when modelcard extraction fails.
// It creates a minimal metadata.yaml file with OCI artifact information and timestamps
// to ensure consistent output structure even for models without embedded modelcards.
//
// Parameters:
//   - manifestRef: container manifest reference for the model
//   - configBlob: container config blob containing timestamp information
func createSkeletonMetadata(manifestRef string, configBlob []byte) {
	// Create output directory
	sanitizedDir := utils.SanitizeManifestRef(manifestRef)
	modelDir := filepath.Join(*outputDir, sanitizedDir, "models")

	err := os.MkdirAll(modelDir, 0755)
	if err != nil {
		log.Printf("  Warning: Failed to create skeleton output directory: %v", err)
		return
	}

	// Create basic metadata with minimal information
	metadata := types.ExtractedMetadata{
		Tags:      []string{}, // Empty tags slice for enrichment to populate
		Language:  []string{},
		Tasks:     []string{},
		Artifacts: registry.ExtractOCIArtifactsFromRegistry(manifestRef),
	}

	// Extract timestamps from config blob if available
	createTime, updateTime := extractTimestampsFromConfig(configBlob)
	for i := range metadata.Artifacts {
		if metadata.Artifacts[i].CreateTimeSinceEpoch == nil {
			metadata.Artifacts[i].CreateTimeSinceEpoch = createTime
		}
		if metadata.Artifacts[i].LastUpdateTimeSinceEpoch == nil {
			metadata.Artifacts[i].LastUpdateTimeSinceEpoch = updateTime
		}
	}

	// Write skeleton metadata.yaml
	metadataFilePath := filepath.Join(modelDir, "metadata.yaml")
	metadataYaml, err := yaml.Marshal(&metadata)
	if err != nil {
		log.Printf("  Warning: Failed to marshal skeleton metadata to YAML: %v", err)
		return
	}

	err = os.WriteFile(metadataFilePath, metadataYaml, 0644)
	if err != nil {
		log.Printf("  Warning: Failed to write skeleton metadata.yaml: %v", err)
		return
	}

	log.Printf("  Successfully created skeleton metadata.yaml: %s", metadataFilePath)
}

// fetchManifestSrcAndLayers retrieves container manifest data and layer information
// from the container registry. It handles Docker reference parsing, image source
// creation, and manifest/layer extraction with comprehensive error handling.
//
// Parameters:
//   - manifestRef: Docker-compatible container manifest reference
//   - sys: container system context for registry authentication
//
// Returns:
//   - containertypes.ImageSource: image source for blob operations
//   - []containertypes.BlobInfo: slice of layer blob information
//   - []byte: container config blob containing metadata
//   - error: registry access or parsing errors, nil on success
func fetchManifestSrcAndLayers(manifestRef string, sys *containertypes.SystemContext) (src containertypes.ImageSource, layers []containertypes.BlobInfo, configBlob []byte, err error) {
	// Create context with timeout for registry operations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Printf("Parsing reference...")
	ref, err := docker.ParseReference("//" + manifestRef)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse reference: %v", err)
	}

	// Create a new image source (later will use to get "the" blob)
	log.Printf("Creating image source...")
	src, err = ref.NewImageSource(ctx, sys)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create image source: %v", err)
	}
	// Ensure src is closed on any subsequent error in this function.
	defer func() {
		if err != nil && src != nil {
			_ = src.Close()
		}
	}()

	// Get the manifest
	manifest, manifestType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get manifest: %v", err)
	}

	log.Printf("Manifest type: %s", manifestType)
	log.Printf("Manifest size: %d bytes", len(manifest))

	// Get the image
	img, err := ref.NewImage(ctx, sys)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create image: %v", err)
	}
	defer func() { _ = img.Close() }()

	// Get the image configuration
	log.Printf("Getting config blob...")
	configBlob, err = img.ConfigBlob(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get config blob: %v", err)
	}

	log.Printf("Config blob size: %d bytes", len(configBlob))

	// Get layer information
	log.Printf("Getting layer infos...")
	layers = img.LayerInfos()
	log.Printf("Number of layers: %d", len(layers))

	// Get layer digests from layer infos
	log.Printf("Layer digests:")
	for i, layer := range layers {
		log.Printf("  Layer %d: %s", i+1, layer.Digest)
	}
	return src, layers, configBlob, nil
}

// OCI Image Config structure for timestamp extraction
type OCIImageConfig struct {
	Created string `json:"created"`
	History []struct {
		Created string `json:"created"`
	} `json:"history"`
}

// parseTimestampWithFallback attempts to parse timestamp strings using multiple formats.
// It tries RFC3339Nano first (most precise), then RFC3339, with comprehensive error handling.
//
// Parameters:
//   - timestampStr: timestamp string to parse
//
// Returns:
//   - *time.Time: parsed time, nil if parsing fails with all supported formats
func parseTimestampWithFallback(timestampStr string) *time.Time {
	if timestampStr == "" {
		return nil
	}

	// Supported timestamp formats in order of preference (most precise first)
	formats := []string{
		time.RFC3339Nano, // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,     // 2006-01-02T15:04:05Z07:00
	}

	for _, format := range formats {
		if parsedTime, err := time.Parse(format, timestampStr); err == nil {
			return &parsedTime
		}
	}

	return nil
}

// extractTimestampsFromConfig extracts creation and update timestamps from OCI config blob
func extractTimestampsFromConfig(configBlob []byte) (*int64, *int64) {
	if len(configBlob) == 0 {
		return nil, nil
	}

	var config OCIImageConfig
	if err := json.Unmarshal(configBlob, &config); err != nil {
		log.Printf("Warning: Failed to parse config blob for timestamps: %v", err)
		return nil, nil
	}

	// Parse creation timestamp
	var createTime *int64
	if config.Created != "" {
		if parsedTime := parseTimestampWithFallback(config.Created); parsedTime != nil {
			epochMs := parsedTime.Unix() * 1000
			createTime = &epochMs
		} else {
			log.Printf("Warning: Failed to parse creation time '%s' with any supported format", config.Created)
		}
	}

	// Use the most recent history entry for update time, fallback to creation time
	updateTime := createTime
	if len(config.History) > 0 {
		lastHistoryEntry := config.History[len(config.History)-1]
		if lastHistoryEntry.Created != "" {
			if parsedTime := parseTimestampWithFallback(lastHistoryEntry.Created); parsedTime != nil {
				epochMs := parsedTime.Unix() * 1000
				updateTime = &epochMs
			}
		}
	}

	log.Printf("Extracted timestamps - Create: %v, Update: %v", formatTimestamp(createTime), formatTimestamp(updateTime))
	return createTime, updateTime
}

// formatTimestamp formats a timestamp pointer for logging
func formatTimestamp(ts *int64) string {
	if ts == nil {
		return "nil"
	}
	return time.Unix(*ts/1000, 0).Format(time.RFC3339)
}

// generateManifestsYAML creates a manifests.yaml file tracking all processed models
func generateManifestsYAML(modelResults []ModelResult, outputDir string) error {
	var manifests types.ManifestsData

	for _, result := range modelResults {
		manifest := types.ModelManifest{
			Ref: result.Ref,
			ModelCard: types.ModelCard{
				Present:  result.ModelCardFound,
				Metadata: result.Metadata,
			},
		}
		manifests.Models = append(manifests.Models, manifest)
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(&manifests)
	if err != nil {
		return err
	}

	// Ensure output directory exists
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return err
	}

	// Write to file in output directory
	manifestsPath := filepath.Join(outputDir, "manifests.yaml")
	err = os.WriteFile(manifestsPath, yamlData, 0644)
	if err != nil {
		return err
	}

	log.Printf("Generated manifests.yaml with %d models", len(manifests.Models))
	return nil
}
