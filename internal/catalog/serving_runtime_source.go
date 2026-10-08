package catalog

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/distribution/reference"
	"github.com/ulikunitz/xz"
	"gopkg.in/yaml.v3"
)

var sourceBlobPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// ErrRuntimeSourceImageUnavailable identifies registry absence or denied access
// during controller/source inspection. It never marks rendering, extraction,
// metadata, or transport errors as skippable.
var ErrRuntimeSourceImageUnavailable = errors.New("controller/source image unavailable")

var defaultRuntimeTemplatePaths = []string{
	"config/overlays/odh/accelerators",
	"config/runtimes",
}

type skopeoInspection struct {
	Digest string            `json:"Digest"`
	Labels map[string]string `json:"Labels"`
}

// RuntimeSourceLoader resolves controller sources before parameter lookups and
// caches pristine extracted configuration for this invocation only. Each load
// renders a private copy using that source's parameters.
type RuntimeSourceLoader struct {
	authFile  string
	workDir   string
	resolved  map[string]string
	extracted map[string]string
}

func NewRuntimeSourceLoader(authFile string) (*RuntimeSourceLoader, error) {
	workDir, err := os.MkdirTemp("", "serving-runtime-source-")
	if err != nil {
		return nil, err
	}
	return &RuntimeSourceLoader{authFile: authFile, workDir: workDir, resolved: map[string]string{}, extracted: map[string]string{}}, nil
}

// Close removes all per-run cached source configuration.
func (l *RuntimeSourceLoader) Close() { _ = os.RemoveAll(l.workDir) }

// Preflight resolves a source to an immutable reference without downloading its
// layers. Successful lookups are reused when templates are loaded later.
func (l *RuntimeSourceLoader) Preflight(ctx context.Context, source RuntimeSourceConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if source.Directory != "" {
		return nil
	}
	key := source.TargetImage + "\x00" + source.SourceImage
	if l.resolved[key] != "" {
		return nil
	}
	image, err := resolveRuntimeSourceImage(ctx, source, l.authFile)
	if err != nil {
		return err
	}
	l.resolved[key] = image
	return nil
}

// Load downloads/extracts each immutable source once per selected set of roots.
// Cache entries are published only after extraction succeeds; rendered copies
// cannot change cached params.env or templates.
func (l *RuntimeSourceLoader) Load(ctx context.Context, source RuntimeSourceConfig) (fs.FS, func(), error) {
	noop := func() {}
	if err := ctx.Err(); err != nil {
		return nil, noop, err
	}
	if source.Directory != "" {
		return renderRuntimeSource(ctx, os.DirFS(source.Directory), source)
	}
	if err := l.Preflight(ctx, source); err != nil {
		return nil, noop, err
	}
	image := l.resolved[source.TargetImage+"\x00"+source.SourceImage]
	roots := source.TemplatePaths
	if len(roots) == 0 {
		roots = defaultRuntimeTemplatePaths
	}
	roots = append(append([]string{}, roots...), "config")
	content := image
	if _, digest, pinned := strings.Cut(image, "@"); pinned && !strings.HasPrefix(image, "dir:") {
		content = digest
	}
	key := content + "\x00" + strings.Join(roots, "\x00")
	templateDir := l.extracted[key]
	if templateDir == "" {
		workDir, err := os.MkdirTemp(l.workDir, "config-")
		if err != nil {
			return nil, noop, err
		}
		imageDir := filepath.Join(workDir, "image")
		if strings.HasPrefix(image, "dir:") {
			imageDir = strings.TrimPrefix(image, "dir:")
		} else if _, err := runRuntimeSkopeo(ctx, l.authFile, "copy", "docker://"+image, "dir:"+imageDir); err != nil {
			_ = os.RemoveAll(workDir)
			return nil, noop, err
		}
		templateDir = filepath.Join(workDir, "templates")
		if err := extractRuntimeSourceImage(ctx, imageDir, templateDir, roots); err != nil {
			_ = os.RemoveAll(workDir)
			return nil, noop, fmt.Errorf("source image %s: %w", image, err)
		}
		if !strings.HasPrefix(image, "dir:") {
			_ = os.RemoveAll(imageDir)
		}
		l.extracted[key] = templateDir
	}
	return renderRuntimeSource(ctx, os.DirFS(templateDir), source)
}

func renderRuntimeSource(ctx context.Context, files fs.FS, source RuntimeSourceConfig) (fs.FS, func(), error) {
	source = discoverRuntimeKustomize(source, files)
	if source.KustomizeOverlay != "" {
		return renderRuntimeKustomize(ctx, files, source)
	}
	if source.ParamsEnv != "" || len(source.Parameters) != 0 {
		return nil, func() {}, fmt.Errorf("parameters require a kustomize overlay; set kustomize_overlay")
	}
	return files, func() {}, nil
}

// LoadServingRuntimeSource loads one source. Multi-source generation shares a
// RuntimeSourceLoader so identical source content is reused during that run.
func LoadServingRuntimeSource(ctx context.Context, source RuntimeSourceConfig, authFile string) (fs.FS, func(), error) {
	loader, err := NewRuntimeSourceLoader(authFile)
	if err != nil {
		return nil, func() {}, err
	}
	files, cleanup, err := loader.Load(ctx, source)
	if err != nil {
		loader.Close()
		return nil, func() {}, err
	}
	return files, func() { cleanup(); loader.Close() }, nil
}

func resolveRuntimeSourceImage(ctx context.Context, source RuntimeSourceConfig, authFile string) (string, error) {
	if strings.HasPrefix(source.SourceImage, "dir:") {
		return source.SourceImage, nil
	}
	if source.SourceImage != "" {
		return pinRuntimeSourceImage(ctx, authFile, source.SourceImage)
	}
	data, err := runRuntimeSkopeo(ctx, authFile, "inspect", "--no-tags", "docker://"+source.TargetImage)
	if err != nil {
		return "", runtimeSourceImageInspectionError(ctx, source.TargetImage, err)
	}
	var inspection skopeoInspection
	if err := json.Unmarshal(data, &inspection); err != nil {
		return "", fmt.Errorf("decode target image inspection: %w", err)
	}
	candidates, err := runtimeSourceImageCandidates(source.TargetImage, inspection)
	if err != nil {
		return "", err
	}
	var failures []string
	allUnavailable := len(candidates) != 0
	for _, candidate := range candidates {
		image, err := pinRuntimeSourceImage(ctx, authFile, candidate)
		if err == nil {
			return image, nil
		}
		failures = append(failures, err.Error())
		if !errors.Is(err, ErrRuntimeSourceImageUnavailable) {
			allUnavailable = false
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	err = fmt.Errorf("cannot resolve source image for %s; configure source_image explicitly: %s", source.TargetImage, strings.Join(failures, "; "))
	if allUnavailable {
		err = fmt.Errorf("%w: %w", ErrRuntimeSourceImageUnavailable, err)
	}
	return "", err
}

func pinRuntimeSourceImage(ctx context.Context, authFile, image string) (string, error) {
	digest, err := inspectRuntimeParameterManifest(ctx, authFile, image)
	if err != nil {
		return "", runtimeSourceImageInspectionError(ctx, image, err)
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", err
	}
	return reference.TrimNamed(ref).Name() + "@" + digest, nil
}

func runtimeSourceImageInspectionError(ctx context.Context, image string, err error) error {
	if ctx.Err() == nil && runtimeImageUnavailable(err, true) {
		return fmt.Errorf("%w: %s: %w", ErrRuntimeSourceImageUnavailable, image, err)
	}
	return err
}

func runRuntimeSkopeo(ctx context.Context, authFile, operation string, args ...string) ([]byte, error) {
	commandArgs := []string{"--override-os", "linux", "--override-arch", "amd64", operation}
	if authFile != "" {
		flag := "--authfile"
		if operation == "copy" {
			flag = "--src-authfile"
		}
		commandArgs = append(commandArgs, flag, authFile)
	}
	for _, arg := range args {
		// Kubernetes accepts tag@digest, but containers/image's Docker transport
		// rejects that spelling. Use the same digest without its informational tag
		// at the skopeo boundary; keep the tag in config and generated manifests.
		if image, ok := strings.CutPrefix(arg, "docker://"); ok {
			if ref, err := reference.ParseNormalizedNamed(image); err == nil {
				if pinned, ok := ref.(reference.Digested); ok {
					arg = "docker://" + reference.TrimNamed(ref).Name() + "@" + pinned.Digest().String()
				}
			}
		}
		commandArgs = append(commandArgs, arg)
	}
	return retryRuntimeRegistryOperation(ctx, "skopeo "+operation, func() ([]byte, error) {
		command := exec.CommandContext(ctx, "skopeo", commandArgs...)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			return nil, fmt.Errorf("skopeo %s: %w: %s", operation, err, strings.TrimSpace(stderr.String()))
		}
		return output, nil
	}, waitRuntimeRegistryRetry)
}

func runtimeSourceImageCandidates(target string, inspection skopeoInspection) ([]string, error) {
	ref, err := reference.ParseNormalizedNamed(target)
	if err != nil {
		return nil, err
	}
	repository := reference.TrimNamed(ref).Name()
	var candidates []string
	// Konflux source attachments use the digest of the resolved binary manifest.
	if sourceBlobPattern.MatchString(inspection.Digest) {
		candidates = append(candidates, repository+":"+strings.ReplaceAll(inspection.Digest, ":", "-")+".src")
	}
	if tagged, ok := ref.(reference.Tagged); ok {
		candidates = append(candidates, repository+":"+tagged.Tag()+"-source")
	}
	if version, release := inspection.Labels["version"], inspection.Labels["release"]; version != "" && release != "" {
		candidate := repository + ":" + version + "-" + release + "-source"
		if len(candidates) == 0 || candidates[len(candidates)-1] != candidate {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

// ResolveRuntimeSourceVersions uses image labels only when the template has no
// runtime version and the deployment image has no tag. Explicit config versions
// and versions stamped by the upstream overlay take precedence.
func ResolveRuntimeSourceVersions(ctx context.Context, source *RuntimeSourceConfig, files fs.FS, authFile string) error {
	source.ImageVersions = map[string]string{}
	roots := runtimeTemplateRoots(*source, files)
	return fs.WalkDir(files, ".", func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !selectedRuntimeFile(file, roots) || (path.Ext(file) != ".yaml" && path.Ext(file) != ".yml") {
			return nil
		}
		if len(source.Include) != 0 {
			included := false
			for _, pattern := range source.Include {
				if yes, _ := path.Match(pattern, file); yes {
					included = true
				}
			}
			if !included {
				return nil
			}
		}
		override := source.Templates[file]
		if override.Version != "" {
			return nil
		}
		data, err := fs.ReadFile(files, file)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		for {
			var manifest map[string]any
			if err := decoder.Decode(&manifest); errors.Is(err, io.EOF) {
				return nil
			} else if err != nil {
				return err
			}
			resource := servingRuntimeResource(manifest)
			if resource == nil {
				if textAt(manifest, "kind") != "LLMInferenceServiceConfig" {
					continue
				}
				resource = manifest
			}
			if len(missingRuntimeManifestImages(manifest, *source, override)) != 0 {
				continue
			}
			if textAt(objectAt(resource, "metadata", "annotations"), "opendatahub.io/runtime-version") != "" {
				continue
			}
			if _, err := replaceRuntimeImages(manifest, runtimeImageReplacements(*source, override)); err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
			image := primaryRuntimeImage(resource)
			if override.Image != "" {
				image = override.Image
			}
			parameterVersion, err := runtimeParameterVersion(source.Parameters, image)
			if err != nil {
				return err
			}
			if parameterVersion != "" {
				continue
			}
			ref, err := reference.ParseNormalizedNamed(image)
			if err != nil {
				return fmt.Errorf("%s: %w", file, err)
			}
			if _, ok := ref.(reference.Tagged); ok || source.ImageVersions[image] != "" {
				continue
			}
			inspectionData, err := runRuntimeSkopeo(ctx, authFile, "inspect", "--no-tags", "docker://"+image)
			if err != nil {
				return fmt.Errorf("%s: resolve runtime image version (or set templates.version): %w", file, err)
			}
			var inspection skopeoInspection
			if err := json.Unmarshal(inspectionData, &inspection); err != nil {
				return err
			}
			version := inspection.Labels["org.opencontainers.image.version"]
			if version == "" {
				version = inspection.Labels["version"]
			}
			if version == "" {
				return fmt.Errorf("%s: image %s has no version label; configure templates.version", file, image)
			}
			source.ImageVersions[image] = version
		}
	})
}

func extractRuntimeSourceImage(ctx context.Context, imageDir, outputDir string, roots []string) error {
	data, err := os.ReadFile(filepath.Join(imageDir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Layers []struct {
			Digest string `json:"digest"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse source image manifest: %w", err)
	}
	if len(manifest.Layers) == 0 {
		return fmt.Errorf("source image has no layers")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, layer := range manifest.Layers {
		if !sourceBlobPattern.MatchString(layer.Digest) {
			return fmt.Errorf("unsupported or invalid layer digest %q", layer.Digest)
		}
		file, err := os.Open(filepath.Join(imageDir, strings.TrimPrefix(layer.Digest, "sha256:")))
		if err != nil {
			return err
		}
		err = scanRuntimeSourceArchive(ctx, file, outputDir, roots, 0, true)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("layer %s: %w", layer.Digest, err)
		}
	}
	return nil
}

// Red Hat source images store source archives in blobs/sha256/* with links in
// extra_src_dir. Those blobs may themselves wrap the repository's tar.gz.
// Stream nested archives and write only bounded YAML files under known roots.
func scanRuntimeSourceArchive(ctx context.Context, input io.Reader, outputDir string, roots []string, depth int, required bool) error {
	if depth > 4 {
		return fmt.Errorf("source archive nesting exceeds four levels")
	}
	reader := bufio.NewReader(input)
	header, _ := reader.Peek(6)
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		decompressed, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer func() { _ = decompressed.Close() }()
		reader = bufio.NewReader(decompressed)
	} else if bytes.HasPrefix(header, []byte("BZh")) {
		reader = bufio.NewReader(bzip2.NewReader(reader))
	} else if bytes.Equal(header, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}) {
		decompressed, err := xz.NewReader(reader)
		if err != nil {
			return err
		}
		reader = bufio.NewReader(decompressed)
	}
	if !required {
		// RPM and other non-archive source blobs are skipped.
		header, _ := reader.Peek(512)
		if len(header) < 512 || !bytes.Equal(header[257:262], []byte("ustar")) {
			return nil
		}
	}
	archive := tar.NewReader(reader)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(entry.Name, "./")
		if !fs.ValidPath(name) {
			return fmt.Errorf("unsafe source archive path %q", entry.Name)
		}
		if normalized := runtimeArchiveTemplatePath(name, roots); normalized != "" && (path.Ext(normalized) == ".yaml" || path.Ext(normalized) == ".yml" || path.Base(normalized) == "params.env") {
			if entry.Size > 4*1024*1024 {
				return fmt.Errorf("template %q exceeds 4 MiB", name)
			}
			data, err := io.ReadAll(archive)
			if err != nil {
				return err
			}
			target := filepath.Join(outputDir, filepath.FromSlash(normalized))
			if existing, err := os.ReadFile(target); err == nil {
				if !bytes.Equal(existing, data) {
					return fmt.Errorf("conflicting source templates at %s", normalized)
				}
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0644); err != nil {
				return err
			}
		} else if strings.HasPrefix(name, "blobs/sha256/") || strings.HasSuffix(name, ".tar") || strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar.xz") || strings.HasSuffix(name, ".tar.bz2") {
			if err := scanRuntimeSourceArchive(ctx, archive, outputDir, roots, depth+1, false); err != nil {
				return fmt.Errorf("nested source archive %s: %w", name, err)
			}
		}
	}
}

func runtimeArchiveTemplatePath(name string, roots []string) string {
	for _, root := range roots {
		if root == "." {
			return name
		}
		if strings.HasPrefix(name, root+"/") {
			return name
		}
		// Repository tarballs have a single project/commit prefix. Matching
		// deeper occurrences would also collect example projects and dependencies.
		if _, relative, ok := strings.Cut(name, "/"); ok && strings.HasPrefix(relative, root+"/") {
			return relative
		}
	}
	return ""
}
