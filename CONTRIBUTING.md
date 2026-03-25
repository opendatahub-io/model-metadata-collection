# Contributing to Model Metadata Collection

Thank you for your interest in contributing to this project.

## Prerequisites

- Go (see `go.mod` for the required version)
- Docker or Podman (for container builds)
- `golangci-lint` (for linting)
- `pre-commit` (optional, for local git hooks)

## Setup

Clone the repository and install dependencies:

```bash
git clone https://github.com/opendatahub-io/model-metadata-collection.git
cd model-metadata-collection
make deps
```

Install development tools:

```bash
make setup
```

This will install `golangci-lint` and `godoc`.

To install pre-commit hooks for local validation:

```bash
pip install pre-commit
pre-commit install
```

## Build

Build the project binary:

```bash
make build
```

For a quick development iteration (format, vet, test, build):

```bash
make dev
```

## Run

Run the model extractor with default settings:

```bash
make run
```

Process all model indexes (Red Hat, validated, other):

```bash
make process
```

Run with a custom input file:

```bash
./build/model-extractor --input path/to/index.yaml --output-dir output/custom
```

## Test

Run the full test suite:

```bash
make test
```

Run tests with coverage reporting:

```bash
make test-coverage
```

Run benchmarks:

```bash
make benchmark
```

### Test Structure

- Unit tests are co-located with source files (`*_test.go`)
- Integration tests in `internal/registry/` are skipped during normal test runs
- Test fixtures are located in `sample-data/` (also accessible via the `testdata/` symlink)

## Debug

To debug issues with model processing:

1. Run the extractor with a single model in isolation:

```bash
./build/model-extractor \
    --input test-index.yaml \
    --output-dir output/debug \
    --skip-catalog \
    --skip-default-static-catalog
```

2. Check extracted metadata:

```bash
cat output/debug/*/models/metadata.yaml
```

3. Verify enrichment by checking name resolution:

```bash
grep "name:" output/debug/*/models/metadata.yaml
```

4. Enable verbose logging by reviewing the `log.Printf` output during processing.

## Code Quality

Before submitting changes, run:

```bash
make check    # Runs fmt-check, vet, lint
make test     # Runs full test suite
```

Or use the full CI pipeline locally:

```bash
make ci       # Runs deps, check, test, build
```

## Commit Conventions

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) standard:

```text
{type}({scope}): {description}

[optional body]

Signed-off-by: Your Name <your.email@example.com>
```

### Types

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `refactor:` - Code refactoring
- `test:` - Test additions or updates
- `chore:` - Maintenance tasks

### DCO Sign-off

All commits must include a DCO sign-off. Use `git commit -s` to add it automatically.

## Pull Request Process

1. Create a feature branch: `feature/{description}` or `fix/{description}`
2. Make your changes with appropriate tests
3. Run `make ci` to verify everything passes
4. Submit a pull request against `main`
