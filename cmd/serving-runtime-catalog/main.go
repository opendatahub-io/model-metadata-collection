package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/opendatahub-io/model-metadata-collection/internal/catalog"
)

func main() {
	inputPath := flag.String("input", "data/redhat-serving-runtimes-index.yaml", "reviewed runtime input")
	outputPath := flag.String("output", "data/redhat-serving-runtimes-catalog.yaml", "generated loader catalog")
	check := flag.Bool("check", false, "verify checked-in output without changing it")
	flag.Parse()
	input, err := os.ReadFile(*inputPath)
	if err == nil {
		var output []byte
		output, err = catalog.GenerateServingRuntimeCatalog(input, os.DirFS("."))
		if err == nil {
			if *check {
				var existing []byte
				existing, err = os.ReadFile(*outputPath)
				if err == nil && !bytes.Equal(existing, output) {
					err = fmt.Errorf("%s is out of date; regenerate it", *outputPath)
				}
			} else {
				err = os.WriteFile(*outputPath, output, 0644)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
