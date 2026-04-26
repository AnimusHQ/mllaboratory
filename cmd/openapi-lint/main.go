package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	var filesArg string
	flag.StringVar(&filesArg, "files", "", "comma-separated list of OpenAPI files")
	flag.Parse()

	files := collectFiles(filesArg, flag.Args())
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no OpenAPI files provided")
		os.Exit(2)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	ctx := context.Background()
	for _, file := range files {
		if strings.TrimSpace(file) == "" {
			continue
		}
		doc, err := loader.LoadFromFile(file)
		if err != nil {
			fail(file, err)
		}
		if err := doc.Validate(ctx); err != nil {
			fail(file, err)
		}
	}
}

func collectFiles(filesArg string, extra []string) []string {
	out := make([]string, 0)
	if strings.TrimSpace(filesArg) != "" {
		for _, part := range strings.Split(filesArg, ",") {
			if val := strings.TrimSpace(part); val != "" {
				out = append(out, val)
			}
		}
	}
	for _, arg := range extra {
		if val := strings.TrimSpace(arg); val != "" {
			out = append(out, val)
		}
	}
	return out
}

func fail(file string, err error) {
	fmt.Fprintf(os.Stderr, "openapi lint failed for %s: %v\n", file, err)
	os.Exit(1)
}
