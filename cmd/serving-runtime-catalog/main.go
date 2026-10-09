package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
)

func main() {
	inputPath := flag.String("input", "data/redhat-serving-runtimes-index.yaml", "reviewed runtime index")
	outputPath := flag.String("output", "data/redhat-serving-runtimes-catalog.yaml", "generated loader catalog")
	configPath := flag.String("config", "", "generator config for discovering templates from checkouts or source images")
	indexOutput := flag.String("index-output", "data/redhat-serving-runtimes-index.yaml", "generated index (with --config)")
	runtimeOutputDir := flag.String("runtime-output-dir", "input/serving_runtimes/generated/redhat", "generated runtime files, relative to repository root (with --config)")
	authFile := flag.String("authfile", "", "registry authentication file for skopeo and cosign (default: normal credential discovery)")
	skipUnavailableImages := flag.Bool("skip-unavailable-images", false, "skip unavailable controller sources and runtime parameter images on registry authorization or not-found errors (with --config)")
	check := flag.Bool("check", false, "verify checked-in output without changing it")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var err error
	if *configPath != "" {
		err = generateFromConfig(ctx, *configPath, *authFile, *runtimeOutputDir, *indexOutput, *outputPath, *check, *skipUnavailableImages)
	} else {
		err = generateFromIndex(*inputPath, *outputPath, *check)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateFromIndex(inputPath, outputPath string, check bool) error {
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	output, err := catalog.GenerateServingRuntimeCatalog(input, os.DirFS("."))
	if err != nil {
		return err
	}
	return writeRuntimeArtifacts(map[string][]byte{outputPath: output}, check)
}

func generateFromConfig(ctx context.Context, configPath, authFile, runtimeOutputDir, indexOutput, catalogOutput string, check, skipUnavailableImages bool) error {
	input, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	config, err := catalog.ParseServingRuntimeGeneratorConfig(input)
	if err != nil {
		return err
	}
	sources := map[string]fs.FS{}
	loader, err := catalog.NewRuntimeSourceLoader(authFile)
	if err != nil {
		return err
	}
	defer loader.Close()
	var availableSources []catalog.RuntimeSourceConfig
	for _, source := range config.Sources {
		fmt.Fprintf(os.Stderr, "Checking controller source %s\n", source.ID)
		if err := loader.Preflight(ctx, source); err != nil {
			if skipUnavailableImages && ctx.Err() == nil && errors.Is(err, catalog.ErrRuntimeSourceImageUnavailable) {
				fmt.Fprintf(os.Stderr, "Skipping source %s: %v\n", source.ID, err)
				continue
			}
			return fmt.Errorf("source %q: %w", source.ID, err)
		}
		availableSources = append(availableSources, source)
	}
	if len(availableSources) == 0 {
		return fmt.Errorf("no controller sources are available; existing outputs were not changed")
	}
	config.Sources = availableSources
	versions := catalog.NewRuntimeSBOMVersionResolver(authFile)
	versions.SkipUnavailableImages = skipUnavailableImages
	for index := range config.Sources {
		fmt.Fprintf(os.Stderr, "Checking parameter images and resolving runtime versions for %s\n", config.Sources[index].ID)
		if err := versions.ResolveParameters(ctx, &config.Sources[index]); err != nil {
			return err
		}
		source := config.Sources[index]
		for _, image := range slices.Sorted(maps.Keys(source.MissingImages)) {
			fmt.Fprintf(os.Stderr, "Source %s: parameter image unavailable: %s\n", source.ID, image)
		}
	}
	// Resolve parameters before downloading or rendering available sources.
	for index := range config.Sources {
		source := config.Sources[index]
		fmt.Fprintf(os.Stderr, "Loading templates from %s\n", source.ID)
		files, cleanup, err := loader.Load(ctx, source)
		if err != nil {
			return fmt.Errorf("source %q: %w", source.ID, err)
		}
		defer cleanup()
		if err := catalog.ResolveRuntimeSourceVersions(ctx, &config.Sources[index], files, authFile); err != nil {
			return fmt.Errorf("source %q: %w", source.ID, err)
		}
		sources[source.ID] = files
	}
	artifacts, err := catalog.GenerateServingRuntimeArtifacts(config, sources, runtimeOutputDir)
	if err != nil {
		return err
	}
	for _, skipped := range artifacts.SkippedTemplates {
		fmt.Fprintf(os.Stderr, "Skipping source %s template %s (%s): unavailable parameter images: %s\n", skipped.SourceID, skipped.Path, skipped.Name, strings.Join(skipped.MissingImages, ", "))
	}
	files := artifacts.RuntimeFiles
	for file, data := range map[string][]byte{indexOutput: artifacts.Index, catalogOutput: artifacts.Catalog} {
		if _, exists := files[file]; exists || indexOutput == catalogOutput {
			return fmt.Errorf("output paths must be distinct: %s", file)
		}
		files[file] = data
	}
	return writeRuntimeArtifacts(files, check)
}

func writeRuntimeArtifacts(files map[string][]byte, check bool) error {
	paths := slices.Sorted(maps.Keys(files))
	if check {
		for _, file := range paths {
			existing, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, files[file]) {
				return fmt.Errorf("%s is out of date; regenerate it", file)
			}
		}
		return nil
	}
	// Stage every file before replacing outputs, so generation/validation errors
	// leave the reviewed artifacts intact.
	temporary := map[string]string{}
	defer func() {
		for _, file := range temporary {
			_ = os.Remove(file)
		}
	}()
	for _, file := range paths {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			return err
		}
		staged, err := os.CreateTemp(filepath.Dir(file), ".serving-runtime-*")
		if err != nil {
			return err
		}
		temporary[file] = staged.Name()
		_, writeErr := staged.Write(files[file])
		closeErr := staged.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Chmod(staged.Name(), 0644); err != nil {
			return err
		}
	}
	for _, file := range paths {
		if err := os.Rename(temporary[file], file); err != nil {
			return err
		}
	}
	return nil
}
