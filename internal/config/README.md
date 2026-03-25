# config

The `config` package provides centralized configuration for the model metadata pipeline.

## Responsibilities

- Defining the single source of truth for supported model families (`SupportedModelFamilies`)
- Providing model family lookup and validation utilities
- Building pre-compiled regex patterns for model name normalization

## Key Exports

- `SupportedModelFamilies` - Alphabetically sorted slice of all recognized model family names
- `IsModelFamily()` - Checks if a token matches a supported model family
- `GetModelFamilyRegexPattern()` - Returns the regex pattern string for model family matching
- `GetModelFamilyRegex()` - Returns the pre-compiled regex for model family matching

## Adding a New Model Family

Add the family name to `SupportedModelFamilies` in alphabetical order, then run `make test` to verify consistency checks pass across all dependent packages.
