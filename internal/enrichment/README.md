# enrichment

The `enrichment` package enriches model metadata by cross-referencing container registry models with HuggingFace data.

## Responsibilities

- Matching registry model names to HuggingFace entries using normalized similarity scoring
- Extracting tool-calling configuration from HuggingFace YAML frontmatter
- Applying vLLM recommended configurations from `input/models/vllm-config/`
- Generating README content sections (tool-calling deployment, vLLM config)
- Preventing cross-family model matching (e.g., llama containers matching granite entries)

## Key Functions

- `EnrichMetadataFromHuggingFace()` - Main enrichment entry point for processed models
- `isCompatibleModelFamily()` - Guards against cross-family matching
- `extractModelFamily()` - Identifies model family from normalized name
- `extractToolCallingMetadata()` - Parses tool-calling fields from YAML frontmatter

## Dependencies

- `internal/config` - Model family definitions
- `internal/huggingface` - HuggingFace data access
- `pkg/utils` - Name normalization and template rendering
