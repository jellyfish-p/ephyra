package openapi

import (
	"strings"
	"testing"

	"github.com/jellyfish-p/ephyra/server/internal/domain"
)

// testResponse stands in for a handler's payload; the builder only needs some
// named struct to reflect.
type testResponse struct {
	OK bool `json:"ok"`
}

// The registry is the only place a route can go missing, so these cover the
// failure modes that would otherwise ship a document quietly describing less
// than the server serves.

func TestBuildRejectsUndocumentedRoute(t *testing.T) {
	_, err := Build(BuildInput{
		Routes: []Route{{Method: "GET", Path: "/api/x", Handler: "h"}},
	})
	if err == nil || !strings.Contains(err.Error(), "registered but undocumented") {
		t.Fatalf("want a missing-spec error, got %v", err)
	}
}

func TestBuildRejectsSpecForUnregisteredRoute(t *testing.T) {
	_, err := Build(BuildInput{
		Specs: []RouteSpec{{Method: "GET", Path: "/api/x", Tag: "t"}},
	})
	if err == nil || !strings.Contains(err.Error(), "documented but not registered") {
		t.Fatalf("want a stray-spec error, got %v", err)
	}
}

func TestBuildRejectsDuplicateSpec(t *testing.T) {
	spec := RouteSpec{Method: "GET", Path: "/api/x", Tag: "t"}
	_, err := Build(BuildInput{
		Routes: []Route{{Method: "GET", Path: "/api/x", Handler: "h"}},
		Specs:  []RouteSpec{spec, spec},
	})
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("want a duplicate-spec error, got %v", err)
	}
}

// A field enum naming a type that was never declared would otherwise document
// an unconstrained string, which is the bug the field overrides exist to fix.
func TestBuildRejectsUndeclaredFieldEnum(t *testing.T) {
	_, err := Build(BuildInput{
		Routes: []Route{{Method: "GET", Path: "/api/x", Handler: "h"}},
		Specs:  []RouteSpec{{Method: "GET", Path: "/api/x", Tag: "t"}},
		Fields: []FieldSpec{{Owner: ErrorResponse{}, Field: "error", EnumType: domain.Rule("")}},
	})
	if err == nil || !strings.Contains(err.Error(), "not declared in Enums") {
		t.Fatalf("want an undeclared-enum error, got %v", err)
	}
}

func TestBuildRejectsFieldEnumWithNoValues(t *testing.T) {
	_, err := Build(BuildInput{
		Routes: []Route{{Method: "GET", Path: "/api/x", Handler: "h"}},
		Specs:  []RouteSpec{{Method: "GET", Path: "/api/x", Tag: "t"}},
		Fields: []FieldSpec{{Owner: ErrorResponse{}, Field: "error"}},
	})
	if err == nil || !strings.Contains(err.Error(), "needs EnumType or Values") {
		t.Fatalf("want a missing-values error, got %v", err)
	}
}

// The success status is part of the contract: a 201 that is documented as a 200
// would have clients treating a created resource as an existing one.
func TestBuildRecordsTheDeclaredStatus(t *testing.T) {
	doc, err := Build(BuildInput{
		Routes: []Route{{Method: "POST", Path: "/api/x", Handler: "h"}},
		Specs: []RouteSpec{{
			Method: "POST", Path: "/api/x", Tag: "t",
			Response: testResponse{}, Status: 201,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	op := doc.Paths["/api/x"].Post
	if _, ok := op.Responses["201"]; !ok {
		t.Fatalf("want a 201 response, got %v", keysOf(op.Responses))
	}
	if _, ok := op.Responses["200"]; ok {
		t.Error("a 200 was emitted for a 201 route")
	}
}

// A multipart route cannot be described by a Go struct, so it has its own path
// through the builder.
func TestBuildDescribesMultipartBody(t *testing.T) {
	doc, err := Build(BuildInput{
		Routes: []Route{{Method: "POST", Path: "/api/x", Handler: "h"}},
		Specs: []RouteSpec{{
			Method: "POST", Path: "/api/x", Tag: "t",
			FormFields: map[string]any{"language": ""},
			FormFiles:  []string{"source"},
			Response:   testResponse{},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := doc.Paths["/api/x"].Post.RequestBody
	media, ok := body.Content["multipart/form-data"]
	if !ok {
		t.Fatalf("want a multipart body, got %v", keysOf(body.Content))
	}
	for _, name := range []string{"language", "source"} {
		if _, ok := media.Schema.Properties[name]; !ok {
			t.Errorf("multipart body is missing %q", name)
		}
	}
	if got := media.Schema.Properties["source"].Format; got != "binary" {
		t.Errorf("file part has format %q, want binary", got)
	}
}

// Gin paths use ":name", which is not a JSON path parameter. The download token
// is the one parameter that is not a uuid.
func TestBuildDescribesPathParameters(t *testing.T) {
	doc, err := Build(BuildInput{
		Routes: []Route{{Method: "GET", Path: "/api/files/:token", Handler: "h"}},
		Specs: []RouteSpec{{
			Method: "GET", Path: "/api/files/:token", Tag: "t",
			PathParams:  map[string]*Schema{"token": {Type: "string"}},
			ContentType: "application/octet-stream",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	params := doc.Paths["/api/files/:token"].Get.Parameters
	if len(params) != 1 || params[0].Name != "token" || params[0].In != "path" {
		t.Fatalf("want one path parameter named token, got %+v", params)
	}
	if params[0].Schema.Format != "" {
		t.Errorf("token should not be a uuid, got format %q", params[0].Schema.Format)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
