package openapi

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

type sampleChild struct {
	ID uuid.UUID `json:"id"`
}

type sampleEmbedded struct {
	Common string `json:"common"`
}

type sample struct {
	sampleEmbedded
	Name     string         `json:"name"`
	Optional *string        `json:"optional,omitempty"`
	Count    int            `json:"count"`
	Ratio    float64        `json:"ratio"`
	Flags    []bool         `json:"flags"`
	Lookup   map[string]int `json:"lookup"`
	Child    sampleChild    `json:"child"`
	ChildPtr *sampleChild   `json:"child_ptr"`
	When     time.Time      `json:"when"`
	Score    exact.Score    `json:"score"`
	Opaque   any            `json:"opaque"`
	Hidden   string         `json:"-"`
	unexp    string         //nolint:unused // proves unexported fields are skipped
}

type node struct {
	Name string `json:"name"`
	Next *node  `json:"next,omitempty"`
}

func TestSchemaOfScalars(t *testing.T) {
	r := NewReflector()
	cases := []struct {
		in   any
		want string
	}{
		{"", "string"},
		{true, "boolean"},
		{int64(0), "integer"},
		{float64(0), "number"},
	}
	for _, c := range cases {
		if got := r.SchemaOf(reflect.TypeOf(c.in)).Type; got != c.want {
			t.Errorf("%T: type %q, want %q", c.in, got, c.want)
		}
	}
	if got := r.SchemaOf(reflect.TypeOf([]string{})).Items.Type; got != "string" {
		t.Errorf("slice items type %q", got)
	}
	if got := r.SchemaOf(reflect.TypeOf(map[string]int{})).AdditionalProperties.Type; got != "integer" {
		t.Errorf("map values type %q", got)
	}
}

// Types whose wire form is not their Go form must not be described by kind.
func TestSchemaOfSpecialTypes(t *testing.T) {
	r := NewReflector()
	if got := r.SchemaOf(reflect.TypeOf(time.Time{})); got.Type != "string" || got.Format != "date-time" {
		t.Errorf("time.Time: %+v", got)
	}
	if got := r.SchemaOf(reflect.TypeOf(uuid.UUID{})); got.Type != "string" || got.Format != "uuid" {
		t.Errorf("uuid.UUID: %+v", got)
	}
	// exact.Score marshals as a decimal string, so a number would be wrong.
	score := r.SchemaOf(reflect.TypeOf(exact.Score{}))
	if score.Type != "string" || score.Format != "decimal" {
		t.Errorf("exact.Score: %+v", score)
	}
	// []byte is base64 text, not an array of integers.
	if got := r.SchemaOf(reflect.TypeOf([]byte{})); got.Type != "string" || got.Format != "byte" {
		t.Errorf("[]byte: %+v", got)
	}
}

func TestSchemaOfStructUsesTagsAndRequired(t *testing.T) {
	r := NewReflector()
	ref := r.SchemaOf(reflect.TypeOf(sample{}))
	if ref.Ref == "" {
		t.Fatal("named struct should be a reference")
	}
	got := r.Schemas()["sample"]
	if got == nil {
		t.Fatal("sample was not registered in components")
	}

	// omitempty fields are optional; everything else is required.
	required := map[string]bool{}
	for _, name := range got.Required {
		required[name] = true
	}
	if required["optional"] {
		t.Error("omitempty field must not be required")
	}
	for _, name := range []string{"name", "count", "child", "when", "score"} {
		if !required[name] {
			t.Errorf("%q should be required", name)
		}
	}
	if _, ok := got.Properties["Hidden"]; ok {
		t.Error(`json:"-" field must be omitted`)
	}
	if _, ok := got.Properties["unexp"]; ok {
		t.Error("unexported field must be omitted")
	}
	// An embedded struct without a json name is flattened, as encoding/json does.
	if _, ok := got.Properties["common"]; !ok {
		t.Error("embedded struct should be flattened into the parent")
	}
}

// A $ref cannot carry siblings in OpenAPI 3.0, so a nullable struct has to be
// wrapped; emitting $ref plus nullable would silently drop the nullable.
func TestNullableNamedStructUsesAllOf(t *testing.T) {
	r := NewReflector()
	got := r.SchemaOf(reflect.TypeOf(&sampleChild{}))
	if got.Ref != "" {
		t.Fatalf("nullable ref must not be a bare $ref: %+v", got)
	}
	if !got.Nullable || len(got.AllOf) != 1 || got.AllOf[0].Ref == "" {
		t.Fatalf("want allOf wrapper with nullable, got %+v", got)
	}
}

func TestNullableScalarKeepsType(t *testing.T) {
	r := NewReflector()
	got := r.SchemaOf(reflect.TypeOf(new(int)))
	if got.Type != "integer" || !got.Nullable {
		t.Fatalf("want nullable integer, got %+v", got)
	}
}

// A self-referential type must terminate: the reflector registers the schema
// before recursing into it.
func TestSelfReferenceTerminates(t *testing.T) {
	r := NewReflector()
	ref := r.SchemaOf(reflect.TypeOf(node{}))
	if ref.Ref == "" {
		t.Fatal("want a reference")
	}
	got := r.Schemas()["node"]
	if got == nil || got.Properties["next"] == nil {
		t.Fatalf("node schema incomplete: %+v", got)
	}
}

func TestEnumIsInlineAndTyped(t *testing.T) {
	r := NewReflector()
	r.Enum(reflect.TypeOf(ErrorCode("")), "unauthorized", "forbidden")
	got := r.SchemaOf(reflect.TypeOf(ErrorCode("")))
	if got.Type != "string" || len(got.Enum) != 2 {
		t.Fatalf("want an inline string enum, got %+v", got)
	}
}

func TestStringConstantsReadsTypedConstants(t *testing.T) {
	values, err := StringConstants("../../internal/domain/domain.go", "Verdict")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"accepted", "wrong_answer", "time_limit_exceeded", "memory_limit_exceeded", "runtime_error", "compilation_error", "system_error"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("verdicts\n got %v\nwant %v", values, want)
	}
}

func TestStringConstantsRejectsUnknownType(t *testing.T) {
	if _, err := StringConstants("../../internal/domain/domain.go", "NoSuchType"); err == nil {
		t.Fatal("an unknown type must be an error, not an empty enum")
	}
	if _, err := StringConstants("../../internal/domain/nope.go", "Verdict"); err == nil {
		t.Fatal("a missing file must be an error")
	}
}
