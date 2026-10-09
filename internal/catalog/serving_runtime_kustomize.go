package catalog

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var runtimeParameterNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// renderRuntimeKustomize stages a private copy, updates params.env, and lets the
// project's own overlay apply replacements (including runtime annotations).
// Only manifests originating in the selected template directories are retained.
func renderRuntimeKustomize(ctx context.Context, files fs.FS, source RuntimeSourceConfig) (fs.FS, func(), error) {
	workDir, err := os.MkdirTemp("", "serving-runtime-kustomize-")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(workDir) }
	fail := func(err error) (fs.FS, func(), error) {
		cleanup()
		return nil, func() {}, err
	}
	// The base/default/overlay trees are siblings under config/ in both projects.
	stageRoot := strings.Split(source.KustomizeOverlay, "/")[0]
	err = fs.WalkDir(files, stageRoot, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		data, err := fs.ReadFile(files, file)
		if err != nil {
			return err
		}
		target := filepath.Join(workDir, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		return fail(fmt.Errorf("stage kustomize source: %w", err))
	}
	paramsFile := source.ParamsEnv
	if paramsFile == "" {
		paramsFile = path.Join(source.KustomizeOverlay, "params.env")
	}
	paramsPath := filepath.Join(workDir, filepath.FromSlash(paramsFile))
	if len(source.Parameters) != 0 {
		data, err := os.ReadFile(paramsPath)
		if err != nil {
			return fail(err)
		}
		updated, err := replaceRuntimeParameters(data, source.Parameters)
		if err != nil {
			return fail(fmt.Errorf("%s: %w", paramsFile, err))
		}
		if err := os.WriteFile(paramsPath, updated, 0644); err != nil {
			return fail(err)
		}
	}
	command := exec.CommandContext(ctx, "kustomize", "build", filepath.Join(workDir, filepath.FromSlash(source.KustomizeOverlay)))
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return fail(fmt.Errorf("kustomize build: %w: %s", err, strings.TrimSpace(stderr.String())))
	}
	rendered := map[string]map[string]any{}
	decoder := yaml.NewDecoder(bytes.NewReader(output))
	for {
		var manifest map[string]any
		if err := decoder.Decode(&manifest); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return fail(err)
		}
		rendered[runtimeManifestID(manifest)] = manifest
	}
	roots := runtimeTemplateRoots(source, files)
	err = fs.WalkDir(files, stageRoot, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !selectedRuntimeFile(file, roots) || (path.Ext(file) != ".yaml" && path.Ext(file) != ".yml") {
			return nil
		}
		data, err := fs.ReadFile(files, file)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var result bytes.Buffer
		encoder := yaml.NewEncoder(&result)
		defer func() { _ = encoder.Close() }()
		changed := false
		for {
			var manifest map[string]any
			if err := decoder.Decode(&manifest); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return err
			}
			if textAt(manifest, "kind") == "LLMInferenceServiceConfig" || servingRuntimeResource(manifest) != nil {
				built, ok := rendered[runtimeManifestID(manifest)]
				if !ok {
					return fmt.Errorf("%s: template %s is absent from the kustomize build", file, runtimeManifestID(manifest))
				}
				manifest = built
				changed = true
			}
			if err := encoder.Encode(manifest); err != nil {
				return err
			}
		}
		if changed {
			return os.WriteFile(filepath.Join(workDir, filepath.FromSlash(file)), result.Bytes(), 0644)
		}
		return nil
	})
	if err != nil {
		return fail(err)
	}
	return os.DirFS(workDir), cleanup, nil
}

func runtimeManifestID(manifest map[string]any) string {
	return textAt(manifest, "kind") + "/" + textAt(objectAt(manifest, "metadata"), "name")
}

func discoverRuntimeKustomize(source RuntimeSourceConfig, files fs.FS) RuntimeSourceConfig {
	if source.KustomizeOverlay != "" {
		return source
	}
	for _, overlay := range []string{"config/overlays/odh", "config/base"} {
		if _, err := fs.Stat(files, path.Join(overlay, "params.env")); err == nil {
			if _, err := fs.Stat(files, path.Join(overlay, "kustomization.yaml")); err == nil {
				source.KustomizeOverlay = overlay
				break
			}
		}
	}
	return source
}

func replaceRuntimeParameters(input []byte, parameters map[string]string) ([]byte, error) {
	remaining := maps.Clone(parameters)
	for _, key := range slices.Sorted(maps.Keys(parameters)) {
		if !runtimeParameterNamePattern.MatchString(key) {
			return nil, fmt.Errorf("invalid params.env key %q", key)
		}
		if strings.ContainsAny(parameters[key], "\r\n") {
			return nil, fmt.Errorf("parameter %q must be a single-line value", key)
		}
	}
	var output strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		key, _, ok := strings.Cut(line, "=")
		if ok && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			key = strings.TrimSpace(key)
			if value, ok := parameters[key]; ok {
				line = key + "=" + value
				delete(remaining, key)
			}
		}
		output.WriteString(line + "\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// A release config can contain parameters introduced after the target image.
	// Append them deterministically; older overlays simply do not reference them.
	for _, key := range slices.Sorted(maps.Keys(remaining)) {
		output.WriteString(key + "=" + remaining[key] + "\n")
	}
	return []byte(output.String()), nil
}
