package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
)

func main() {
	input := flag.String("catalog", "data/redhat-serving-runtimes-catalog.yaml", "loader catalog to validate against the pinned model-registry API schema")
	flag.Parse()
	data, err := os.ReadFile(*input)
	var result *catalog.ServingRuntimeSchemaValidation
	if err == nil {
		result, err = catalog.ValidateServingRuntimeCatalogSchema(context.Background(), data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d runtimes, %d versions validated against model-registry schemas at %s\n", *input, result.Runtimes, result.Versions, catalog.ServingRuntimeSchemaRevision)
	for _, field := range result.CatalogExtensions {
		fmt.Printf("Note: %s is validated locally as a catalog extension; the pinned upstream API and loader do not expose it.\n", field)
	}
}
