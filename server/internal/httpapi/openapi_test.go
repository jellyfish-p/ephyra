package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
)

// The document is committed and CI fails when it drifts, so it has to be a pure
// function of the code. A map iterated anywhere in the builder would make the
// staleness check flap between runs.
func TestOpenAPIDocumentIsDeterministic(t *testing.T) {
	first, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("OpenAPIDocument: %v", err)
	}
	second, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("OpenAPIDocument: %v", err)
	}
	a, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("two builds of the document differ; something iterates a map into the output")
	}
}

// Build refuses to run when the registry and the router disagree, which is what
// makes an undocumented handler impossible to merge.
func TestOpenAPIDocumentCoversEveryRoute(t *testing.T) {
	doc, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("OpenAPIDocument: %v", err)
	}

	registered := (&API{}).Router().Routes()
	operations := 0
	for _, p := range doc.Paths {
		operations += len(p.Operations())
	}
	if operations != len(registered) {
		t.Fatalf("documented %d operations, router serves %d", operations, len(registered))
	}
}

func TestOpenAPIOperationsAreComplete(t *testing.T) {
	doc, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("OpenAPIDocument: %v", err)
	}

	tags := map[string]bool{}
	for _, tag := range doc.Tags {
		tags[tag.Name] = true
	}

	for path, item := range doc.Paths {
		for method, op := range item.Operations() {
			where := method + " " + path
			if op.Summary == "" {
				t.Errorf("%s: no summary", where)
			}
			if op.OperationID == "" {
				t.Errorf("%s: no operationId", where)
			}
			if len(op.Tags) != 1 || !tags[op.Tags[0]] {
				t.Errorf("%s: tag %v is not one of the declared tags", where, op.Tags)
			}
			if len(op.Responses) == 0 {
				t.Errorf("%s: no responses", where)
			}
			if _, ok := op.Responses["default"]; !ok {
				t.Errorf("%s: no default response for the error envelope", where)
			}

			// A path parameter the template does not mention would be
			// unsatisfiable for a client.
			for _, p := range op.Parameters {
				if p.In == "path" && !strings.Contains(path, ":"+p.Name) {
					t.Errorf("%s: path parameter %q is not in the template", where, p.Name)
				}
			}

			// Public routes must not advertise a session they do not need, and
			// private ones must say how to authenticate.
			if len(op.Security) == 0 {
				if _, ok := op.Responses["401"]; ok {
					t.Errorf("%s: unauthenticated but documents a 401", where)
				}
				continue
			}
			if _, ok := op.Responses["401"]; !ok {
				t.Errorf("%s: requires a session but documents no 401", where)
			}
		}
	}
}

// The frontend branches on these unions, so losing one would silently widen a
// type rather than break a test elsewhere.
func TestOpenAPIEnumsReachTheWireFields(t *testing.T) {
	doc, err := OpenAPIDocument()
	if err != nil {
		t.Fatalf("OpenAPIDocument: %v", err)
	}

	for _, tc := range []struct {
		schema string
		field  string
		values []string
	}{
		{"SubmissionPublic", "verdict", []string{"accepted", "compilation_error", "system_error"}},
		{"SubmissionPublic", "feedback", []string{"none", "compile", "score", "full"}},
		{"SubmissionPublic", "language_id", []string{"cpp23", "c17", "c23"}},
		{"contestResponse", "rule", []string{"acm", "oi", "ioi"}},
		{"contestResponse", "entry_mode", []string{"individual", "team"}},
		{"TaskView", "state", []string{"pending_schedule", "awaiting_admin"}},
		{"TaskView", "purpose", []string{"initial", "rejudge"}},
		{"AttemptView", "state", []string{"dispatched", "succeeded", "failed", "invalidated"}},
		{"JoinRequest", "status", []string{"pending", "approved", "rejected"}},
	} {
		schema, ok := doc.Components.Schemas[tc.schema]
		if !ok {
			t.Errorf("schema %s is missing", tc.schema)
			continue
		}
		prop, ok := schema.Properties[tc.field]
		if !ok {
			t.Errorf("%s.%s is missing", tc.schema, tc.field)
			continue
		}
		for _, want := range tc.values {
			if !contains(prop.Enum, want) {
				t.Errorf("%s.%s: enum %v is missing %q", tc.schema, tc.field, prop.Enum, want)
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
