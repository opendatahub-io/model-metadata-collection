package catalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func sourceTestArchive(t *testing.T, files map[string][]byte, compress bool) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := tar.NewWriter(&output)
	for name, data := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if compress {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write(output.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return compressed.Bytes()
	}
	return output.Bytes()
}

func TestExtractRuntimeSourceImageNestedArchives(t *testing.T) {
	directory := t.TempDir()
	repository := sourceTestArchive(t, map[string][]byte{
		"kserve-commit/config/overlays/odh/accelerators/nvidia.yaml":    []byte(discoveredLLMConfig),
		"kserve-commit/config/overlays/odh/params.env":                  []byte("kserve-llm-d-nvidia-cuda=registry.redhat.io/rhoai/vllm:1.0\n"),
		"kserve-commit/config/rbac/kustomization.yaml":                  []byte("resources: []\n"),
		"kserve-commit/examples/project/config/rbac/kustomization.yaml": []byte("unrelated: true\n"),
		"kserve-commit/main.go":                                         []byte("package main"),
	}, true)
	wrapper := sourceTestArchive(t, map[string][]byte{"./kserve-commit.tar.gz": repository}, false)
	layer := sourceTestArchive(t, map[string][]byte{
		"./blobs/sha256/source": wrapper,
		"./blobs/sha256/rpm":    {0xed, 0xab, 0xee, 0xdb},
	}, true)
	digest := strings.Repeat("a", 64)
	manifest, _ := json.Marshal(map[string]any{"layers": []any{map[string]any{"digest": "sha256:" + digest}}})
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), manifest, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, digest), layer, 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "templates")
	if err := extractRuntimeSourceImage(context.Background(), directory, output, []string{"config"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "config/overlays/odh/accelerators/nvidia.yaml"))
	if err != nil || string(data) != discoveredLLMConfig {
		t.Fatalf("template extraction changed data: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "main.go")); !os.IsNotExist(err) {
		t.Fatal("non-template code was extracted")
	}
	if _, err := os.Stat(filepath.Join(output, "config/overlays/odh/params.env")); err != nil {
		t.Fatal("params.env was not extracted")
	}
}

func TestScanRuntimeSourceArchiveRejectsTraversalAndConflicts(t *testing.T) {
	output := t.TempDir()
	traversal := sourceTestArchive(t, map[string][]byte{"../outside.yaml": []byte("data")}, false)
	if err := scanRuntimeSourceArchive(context.Background(), bytes.NewReader(traversal), output, []string{"config"}, 0, true); err == nil {
		t.Fatal("accepted path traversal")
	}
	for _, value := range []string{"first", "conflicting"} {
		archive := sourceTestArchive(t, map[string][]byte{"repo/config/runtimes/vllm.yaml": []byte(value)}, false)
		err := scanRuntimeSourceArchive(context.Background(), bytes.NewReader(archive), output, []string{"config"}, 0, true)
		if value == "first" && err != nil {
			t.Fatal(err)
		}
		if value != "first" && (err == nil || !strings.Contains(err.Error(), "conflicting")) {
			t.Fatalf("expected conflicting template error: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := scanRuntimeSourceArchive(ctx, bytes.NewReader(traversal), output, []string{"config"}, 0, true); err == nil {
		t.Fatal("cancellation was ignored")
	}
}

func TestRuntimeSourceImageCandidates(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	inspection := skopeoInspection{Digest: digest, Labels: map[string]string{"version": "v3.6", "release": "123"}}
	got, err := runtimeSourceImageCandidates("registry.redhat.io/rhoai/controller:v3.6-123", inspection)
	want := []string{"registry.redhat.io/rhoai/controller:sha256-" + strings.Repeat("a", 64) + ".src", "registry.redhat.io/rhoai/controller:v3.6-123-source"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v: %v", got, want, err)
	}
	got, err = runtimeSourceImageCandidates("registry.redhat.io/rhoai/controller@"+digest, inspection)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("digest target did not use version/release fallback: %v: %v", got, err)
	}
}

func TestRuntimeSkopeoAcceptsTagAndDigest(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "skopeo"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "docker://quay.io/rhoai/controller:rhoai-3.6@" + digest
	output, err := runRuntimeSkopeo(context.Background(), "", "inspect", "--no-tags", image)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "docker://quay.io/rhoai/controller@"+digest) || strings.Contains(string(output), "rhoai-3.6@") {
		t.Fatalf("skopeo received an unsupported tag-and-digest reference: %s", output)
	}
}

func TestResolveRuntimeSourceVersionsFromImageLabels(t *testing.T) {
	root := t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in\n*'inspect --no-tags docker://registry.redhat.io/rhoai/vllm@sha256:'*) printf '%s' '{\"Labels\":{\"org.opencontainers.image.version\":\"0.22.0\"}}';;\n*) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(root, "skopeo"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	image := "registry.redhat.io/rhoai/vllm@sha256:" + strings.Repeat("a", 64)
	source := RuntimeSourceConfig{ID: "source", Replacements: map[string]string{"placeholder": image}}
	files := fstest.MapFS{"config/runtimes/cuda.yaml": {Data: []byte(discoveredLLMConfig)}}
	if err := ResolveRuntimeSourceVersions(context.Background(), &source, files, ""); err != nil {
		t.Fatal(err)
	}
	if source.ImageVersions[image] != "0.22.0" {
		t.Fatalf("version was not resolved from the image label: %v", source.ImageVersions)
	}
	config := &ServingRuntimeGeneratorConfig{Source: "Runtimes", Provider: "Red Hat", Sources: []RuntimeSourceConfig{source}}
	if _, err := GenerateServingRuntimeArtifacts(config, map[string]fs.FS{source.ID: files}, "generated"); err != nil {
		t.Fatal(err)
	}
	// A missing init image excludes this template even though its primary image
	// is available; metadata discovery must not query the primary image's labels.
	if err := os.WriteFile(filepath.Join(root, "skopeo"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	source.MissingImages = map[string]bool{"registry.redhat.io/rhoai/helper:2.0": true}
	if err := ResolveRuntimeSourceVersions(context.Background(), &source, files, ""); err != nil || len(source.ImageVersions) != 0 {
		t.Fatalf("skipped template triggered label discovery: %v, %v", source.ImageVersions, err)
	}
}

func TestRuntimeSourceLoaderCachesPristineConfigurationPerRun(t *testing.T) {
	for _, aliases := range []bool{false, true} {
		t.Run(fmt.Sprintf("sourceAliases=%t", aliases), func(t *testing.T) {
			root := t.TempDir()
			imageDir := filepath.Join(root, "source-image")
			if err := os.Mkdir(imageDir, 0755); err != nil {
				t.Fatal(err)
			}
			params := "runtime-image=registry.redhat.io/rhaii/runtime:original\n"
			layer := sourceTestArchive(t, map[string][]byte{
				"repo/config/runtimes/runtime.yaml":   []byte(discoveredTemplate),
				"repo/config/base/kustomization.yaml": []byte("kind: Kustomization\n"),
				"repo/config/base/params.env":         []byte(params),
			}, true)
			layerDigest := fmt.Sprintf("%x", sha256.Sum256(layer))
			manifest, err := json.Marshal(map[string]any{"schemaVersion": 2, "layers": []any{map[string]any{"digest": "sha256:" + layerDigest}}})
			if err != nil {
				t.Fatal(err)
			}
			for file, data := range map[string][]byte{"manifest.json": manifest, layerDigest: layer} {
				if err := os.WriteFile(filepath.Join(imageDir, file), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			skopeo := `#!/bin/sh
printf '%s\n' "$*" >> "$SOURCE_CACHE_TEST_LOG"
case "$*" in
  *'inspect --no-tags'*) printf '{"Digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","Labels":{}}' ;;
  *'inspect --raw'*) cat "$SOURCE_CACHE_TEST_IMAGE/manifest.json" ;;
  *'copy'*)
    for arg; do destination="$arg"; done
    destination="${destination#dir:}"
    mkdir -p "$destination"
    cp "$SOURCE_CACHE_TEST_IMAGE/"* "$destination/"
    ;;
  *) exit 2 ;;
esac
`
			kustomize := `#!/bin/sh
while IFS='=' read -r key value; do
  if [ "$key" = runtime-image ]; then image="$value"; fi
done < "$2/params.env"
cat <<EOF
apiVersion: template.openshift.io/v1
kind: Template
metadata: {name: vllm-cuda-runtime-template}
objects:
  - apiVersion: serving.kserve.io/v1alpha1
    kind: ServingRuntime
    metadata: {name: vllm-cuda}
    spec:
      containers:
        - name: kserve-container
          image: $image
EOF
`
			for name, script := range map[string]string{"skopeo": skopeo, "kustomize": kustomize} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			logFile := filepath.Join(root, "commands")
			t.Setenv("PATH", root+":"+os.Getenv("PATH"))
			t.Setenv("SOURCE_CACHE_TEST_IMAGE", imageDir)
			t.Setenv("SOURCE_CACHE_TEST_LOG", logFile)
			loader, err := NewRuntimeSourceLoader("")
			if err != nil {
				t.Fatal(err)
			}
			defer loader.Close()
			var loaded []fs.FS
			for i := range 2 {
				source := RuntimeSourceConfig{ID: fmt.Sprintf("release-%d", i), TargetImage: "registry.redhat.io/rhoai/controller:v3.6", Parameters: map[string]string{"runtime-image": fmt.Sprintf("registry.redhat.io/rhaii/runtime:%d.0", i+1)}}
				if aliases {
					source.TargetImage = ""
					source.SourceImage = fmt.Sprintf("registry.redhat.io/rhoai/source-%d:alias", i)
				}
				if err := loader.Preflight(context.Background(), source); err != nil {
					t.Fatal(err)
				}
				files, cleanup, err := loader.Load(context.Background(), source)
				if err != nil {
					t.Fatal(err)
				}
				defer cleanup()
				loaded = append(loaded, files)
			}
			for i, files := range loaded {
				data, err := fs.ReadFile(files, "config/runtimes/runtime.yaml")
				if err != nil || !strings.Contains(string(data), fmt.Sprintf("registry.redhat.io/rhaii/runtime:%d.0", i+1)) {
					t.Fatalf("parameter sets contaminated one another: %s, %v", data, err)
				}
			}
			log, err := os.ReadFile(logFile)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(log), " copy ") != 1 || len(loader.extracted) != 1 {
				t.Fatalf("identical source was downloaded/extracted more than once: %s", log)
			}
			sourceDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(manifest))
			if !strings.Contains(string(log), "@"+sourceDigest+" dir:") {
				t.Fatalf("download was not pinned to resolved source content: %s", log)
			}
			if !aliases && (strings.Count(string(log), "inspect --no-tags") != 1 || strings.Count(string(log), "inspect --raw") != 1) {
				t.Fatalf("preflight lookups were repeated during loading: %s", log)
			}
			for _, directory := range loader.extracted {
				data, err := os.ReadFile(filepath.Join(directory, "config/base/params.env"))
				if err != nil || string(data) != params {
					t.Fatalf("cached source defaults changed: %q, %v", data, err)
				}
			}
			loader.Close()
			if _, err := os.Stat(loader.workDir); !os.IsNotExist(err) {
				t.Fatalf("ephemeral cache survived cleanup: %v", err)
			}
			// A new invocation has no cache and must download/extract again.
			fresh, err := NewRuntimeSourceLoader("")
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.Close()
			_, cleanup, err := fresh.Load(context.Background(), RuntimeSourceConfig{SourceImage: "registry.redhat.io/rhoai/source:alias-0"})
			if err != nil {
				t.Fatal(err)
			}
			cleanup()
			log, err = os.ReadFile(logFile)
			if err != nil || strings.Count(string(log), " copy ") != 2 {
				t.Fatalf("a new invocation reused a persistent cache: %s, %v", log, err)
			}
		})
	}
}
