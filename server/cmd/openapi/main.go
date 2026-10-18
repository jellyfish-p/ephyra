// Command openapi writes the API document the frontend types are generated
// from.
//
//	go run ./cmd/openapi            # write docs/openapi.json
//	go run ./cmd/openapi -check     # fail if it is out of date
//
// The -check form is what CI runs. Comparing in-process rather than diffing
// against git keeps the check honest in a shallow clone and gives a useful
// message instead of "files differ".
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jellyfish-p/ephyra/server/internal/httpapi"
)

func main() {
	out := flag.String("o", filepath.Join("docs", "openapi.json"), "where to write the document")
	check := flag.Bool("check", false, "verify the document is up to date instead of writing it")
	flag.Parse()

	// Only the route table is wanted here, so keep Gin from printing it.
	gin.SetMode(gin.ReleaseMode)

	doc, err := httpapi.OpenAPIDocument()
	if err != nil {
		fail(err)
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')

	if *check {
		existing, err := os.ReadFile(*out)
		if err != nil {
			fail(fmt.Errorf("reading %s: %w", *out, err))
		}
		if !bytes.Equal(existing, data) {
			fail(fmt.Errorf("%s is out of date; run: go run ./cmd/openapi", *out))
		}
		fmt.Printf("%s is up to date\n", *out)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(data))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "openapi:", err)
	os.Exit(1)
}
