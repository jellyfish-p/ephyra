// Package openapi builds an OpenAPI document from the live Gin route table and
// the Go types the handlers actually use.
//
// The point is that nothing here is a second copy of the API: paths come from
// the router the server really serves, and schemas come from the same structs
// the handlers marshal. A route or field that drifts shows up as a diff in the
// generated file rather than as documentation that quietly lies.
package openapi

import (
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

// Types whose wire shape differs from their Go shape, so reflection alone would
// describe them wrongly.
var (
	timeType  = reflect.TypeOf(time.Time{})
	uuidType  = reflect.TypeOf(uuid.UUID{})
	scoreType = reflect.TypeOf(exact.Score{})
)

// Schema is a JSON Schema as OpenAPI 3.0 uses it. Nullability is a sibling flag
// rather than a type union, and a $ref cannot carry siblings, which is what
// AllOf is for.
type Schema struct {
	Ref                  string             `json:"$ref,omitempty"`
	AllOf                []*Schema          `json:"allOf,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Enum                 []string           `json:"enum,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
}

// Reflector turns Go types into Schemas, collecting every named struct it meets
// into the document's component schemas so they are defined once and referenced.
type Reflector struct {
	schemas map[string]*Schema
	enums   map[reflect.Type][]string
	names   map[reflect.Type]string
	fields  map[fieldKey]*Schema
	seen    map[reflect.Type]bool
}

// fieldKey identifies one field of one struct, for schema overrides.
type fieldKey struct {
	owner reflect.Type
	name  string
}

func NewReflector() *Reflector {
	return &Reflector{
		schemas: map[string]*Schema{},
		enums:   map[reflect.Type][]string{},
		names:   map[reflect.Type]string{},
		fields:  map[fieldKey]*Schema{},
		seen:    map[reflect.Type]bool{},
	}
}

// Field replaces the schema for one field of a struct.
//
// The wire types in this API carry several fields as a bare string even though
// only a known set of values is ever written, so the enum cannot be attached to
// the Go type. Declaring it per field keeps the document accurate without
// changing what the handlers marshal.
func (r *Reflector) Field(owner reflect.Type, name string, s *Schema) *Reflector {
	r.fields[fieldKey{owner, name}] = s
	return r
}

// Enum fixes the allowed values for a named string type. Without it a domain
// constant type would be documented as a bare string, which loses the one thing
// a generated client most needs.
func (r *Reflector) Enum(t reflect.Type, values ...string) *Reflector {
	r.enums[t] = values
	return r
}

// Name overrides the component name for a type, for when two packages use the
// same short name.
func (r *Reflector) Name(t reflect.Type, name string) *Reflector {
	r.names[t] = name
	return r
}

func (r *Reflector) Schemas() map[string]*Schema { return r.schemas }

// SchemaOf describes a type as it appears on the wire. Pointers make the result
// nullable, and a pointer to a named struct is wrapped in allOf because a $ref
// cannot carry a sibling keyword.
func (r *Reflector) SchemaOf(t reflect.Type) *Schema {
	nullable := false
	for t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}
	s := r.schemaFor(t)
	if !nullable {
		return s
	}
	if s.Ref == "" {
		clone := *s
		clone.Nullable = true
		return &clone
	}
	return &Schema{AllOf: []*Schema{s}, Nullable: true}
}

func (r *Reflector) schemaFor(t reflect.Type) *Schema {
	switch t {
	case timeType:
		return &Schema{Type: "string", Format: "date-time"}
	case uuidType:
		return &Schema{Type: "string", Format: "uuid"}
	case scoreType:
		// exact.Score marshals as a decimal string, never a number, so a client
		// that parses it as a float would be wrong.
		return &Schema{Type: "string", Format: "decimal"}
	}

	if values, ok := r.enums[t]; ok {
		return &Schema{Type: "string", Enum: values}
	}

	switch t.Kind() {
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string", Format: "byte"}
		}
		return &Schema{Type: "array", Items: r.SchemaOf(t.Elem())}
	case reflect.Map:
		return &Schema{Type: "object", AdditionalProperties: r.SchemaOf(t.Elem())}
	case reflect.Struct:
		return r.refFor(t)
	default:
		// interface{}, json.RawMessage and anything else opaque.
		return &Schema{}
	}
}

// refFor registers a named struct in the components and returns a reference to
// it. The seen guard is set before recursing so a self-referential type
// terminates instead of building an infinite schema.
func (r *Reflector) refFor(t reflect.Type) *Schema {
	name := r.nameFor(t)
	if !r.seen[t] {
		r.seen[t] = true
		r.schemas[name] = r.structSchema(t)
	}
	return &Schema{Ref: "#/components/schemas/" + name}
}

func (r *Reflector) nameFor(t reflect.Type) string {
	if n, ok := r.names[t]; ok {
		return n
	}
	if t.Name() != "" {
		return t.Name()
	}
	return t.String()
}

func (r *Reflector) structSchema(t reflect.Type) *Schema {
	s := &Schema{Type: "object", Properties: map[string]*Schema{}}
	r.collectFields(t, s, map[string]bool{})
	if len(s.Properties) == 0 {
		s.Properties = nil
	}
	sort.Strings(s.Required)
	return s
}

// HasField reports whether t marshals a field under name, counting fields
// promoted from embedded structs.
//
// It exists so the builder can reject a field override that names a field which
// is not there. Without it such an override is a silent no-op, and the document
// would quietly keep documenting an unconstrained string.
func HasField(t reflect.Type, name string) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		got, _ := jsonName(f)
		if got == "-" {
			continue
		}
		if f.Anonymous && got == "" {
			embedded := f.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct && embedded != timeType {
				if HasField(embedded, name) {
					return true
				}
				continue
			}
		}
		if got == "" {
			got = f.Name
		}
		if got == name {
			return true
		}
	}
	return false
}

// collectFields walks a struct, flattening embedded structs that carry no json
// name of their own, which is how encoding/json treats them too.
func (r *Reflector) collectFields(t reflect.Type, s *Schema, done map[string]bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue // unexported
		}
		name, opts := jsonName(f)
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			embedded := f.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct && embedded != timeType {
				r.collectFields(embedded, s, done)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		if done[name] {
			continue
		}
		done[name] = true
		if override, ok := r.fields[fieldKey{t, name}]; ok {
			s.Properties[name] = override
		} else {
			s.Properties[name] = r.SchemaOf(f.Type)
		}
		if !strings.Contains(opts, "omitempty") {
			s.Required = append(s.Required, name)
		}
	}
}

func jsonName(f reflect.StructField) (name, opts string) {
	tag := f.Tag.Get("json")
	if tag == "" {
		return "", ""
	}
	name, opts, _ = strings.Cut(tag, ",")
	return name, opts
}
