# huggingface

The `huggingface` package provides the HuggingFace API client and collection processing logic.

## Responsibilities

- Discovering Red Hat AI validated model collections via the HuggingFace API
- Fetching collection metadata and model listings
- Parsing version information from collection titles (semver and date-based)
- Generating version-specific index files (`data/hugging-face-redhat-ai-validated-v*.yaml`)
- Fetching HuggingFace README content for metadata enrichment

## Key Functions

- `FetchCollections()` - Queries the HuggingFace API for collections
- `DiscoverValidatedModelCollections()` - Filters collections matching validated model patterns
- `ProcessCollections()` - Orchestrates collection discovery, fetching, and index file generation
- `parseVersionFromTitle()` - Extracts version identifiers from collection titles
- `GetLatestVersionIndexFile()` - Finds the most recent version-specific index file
