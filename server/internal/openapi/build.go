package openapi

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

// Route is one entry from the router the server actually serves. Paths and
// methods come from here rather than from the registry, so a handler that is
// registered but undocumented fails the build instead of being forgotten.
type Route struct {
	Method  string
	Path    string
	Handler string
}

// RouteSpec declares the parts of a route that reflection cannot recover: what
// it accepts, what it returns, and how to describe it.
type RouteSpec struct {
	Method  string
	Path    string
	Summary string
	Tag     string

	// Response is a zero value of the success payload type, or nil when the
	// route returns no JSON body.
	Response any
	// EnvelopeKey wraps Response in an object under this key, matching the
	// object the handler writes. Empty means the payload is the whole body.
	EnvelopeKey string
	// Request is a zero value of the request body type, or nil.
	Request any
	// Query maps each supported query parameter to a zero value of its type.
	// Every one is optional; the API treats them as filters.
	Query map[string]any
	// Status overrides the success status. Defaults to 200.
	Status int
	// Public marks a route that needs no session.
	Public bool
	// ContentType marks a success response that is not JSON, such as an SSE
	// stream or a file download, so no schema is invented for it.
	ContentType string
	// FormFields documents a multipart/form-data body, mapping each text field
	// to a zero value of its type.
	FormFields map[string]any
	// FormFiles names the file parts of a multipart/form-data body.
	FormFiles []string
	// PathParams overrides the schema of a path parameter. Anything not listed
	// is a uuid, which is every parameter except the judge download token.
	PathParams map[string]*Schema
}

// EnumSource points at a Go file and a named string type inside it. Values are
// read from the source so adding a constant updates the document.
type EnumSource struct {
	File string
	Type string
	// Target is a zero value of the Go type to attach the enum to.
	Target any
}

// FieldSpec attaches an enum to one field of a struct, for the fields whose Go
// type is a bare string even though only a known set of values is written.
// Exactly one of EnumType and Values is set.
type FieldSpec struct {
	Owner    any
	Field    string
	EnumType any // a zero value of a type already declared in Enums
	// Values lists the accepted values directly, for the handful of fields whose
	// values exist only as literals in the handlers.
	Values      []string
	Description string
}

type BuildInput struct {
	Title       string
	Version     string
	Description string
	Servers     []Server
	Tags        []Tag
	Routes      []Route
	Specs       []RouteSpec
	Enums       []EnumSource
	Fields      []FieldSpec
}

// ErrorCode is the machine-readable half of the error envelope.
type ErrorCode string

type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// ErrorResponse is the envelope writeErr produces for every failure.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

const (
	cookieScheme = "sessionCookie"
	csrfScheme   = "csrfToken"
)

// Build assembles the document. It fails rather than emitting a partial one
// whenever the registry and the router disagree, because a document that
// silently omits routes is worse than no document.
func Build(in BuildInput) (*Document, error) {
	root, err := serverRoot()
	if err != nil {
		return nil, err
	}

	r := NewReflector()
	for _, e := range in.Enums {
		values, err := StringConstants(filepath.Join(root, e.File), e.Type)
		if err != nil {
			return nil, fmt.Errorf("enum %s: %w", e.Type, err)
		}
		r.Enum(reflect.TypeOf(e.Target), values...)
	}
	// Resolve field enums after the type enums, so a typo in EnumType fails the
	// build instead of silently documenting an unconstrained string.
	for _, f := range in.Fields {
		values := f.Values
		if f.EnumType != nil {
			found, ok := r.enums[reflect.TypeOf(f.EnumType)]
			if !ok {
				return nil, fmt.Errorf("field %s: enum type %T is not declared in Enums", f.Field, f.EnumType)
			}
			values = found
		}
		if len(values) == 0 {
			return nil, fmt.Errorf("field %s: needs EnumType or Values", f.Field)
		}
		owner := reflect.TypeOf(f.Owner)
		if !HasField(owner, f.Field) {
			return nil, fmt.Errorf("field %s: %s has no field with that json name", f.Field, owner)
		}
		r.Field(owner, f.Field, &Schema{
			Type: "string", Enum: values, Description: f.Description,
		})
	}

	specs := make(map[string]RouteSpec, len(in.Specs))
	for _, s := range in.Specs {
		key := routeKey(s.Method, s.Path)
		if _, dup := specs[key]; dup {
			return nil, fmt.Errorf("registry lists %s twice", key)
		}
		specs[key] = s
	}

	seen := make(map[string]bool, len(in.Routes))
	var missing, extra []string
	for _, route := range in.Routes {
		key := routeKey(route.Method, route.Path)
		seen[key] = true
		if _, ok := specs[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range specs {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		return nil, fmt.Errorf(
			"registry does not match the router\n  registered but undocumented: %s\n  documented but not registered: %s",
			strings.Join(missing, ", "), strings.Join(extra, ", "))
	}

	doc := &Document{
		OpenAPI: "3.0.3",
		Info:    Info{Title: in.Title, Version: in.Version, Description: in.Description},
		Servers: in.Servers,
		Tags:    in.Tags,
		Paths:   map[string]*PathItem{},
		Components: Components{
			SecuritySchemes: map[string]*SecurityScheme{
				cookieScheme: {
					Type: "apiKey", In: "cookie", Name: "ephyra_session",
					Description: "Session cookie. GET /api/auth/csrf first, then POST /api/auth/login.",
				},
				csrfScheme: {
					Type: "apiKey", In: "header", Name: "X-CSRF-Token",
					Description: "Must echo the readable ephyra_csrf cookie. Required on every unsafe method.",
				},
			},
		},
	}

	// Sort routes so the output is stable and diffs stay meaningful.
	order := append([]Route(nil), in.Routes...)
	sort.Slice(order, func(i, j int) bool {
		if order[i].Path != order[j].Path {
			return order[i].Path < order[j].Path
		}
		return order[i].Method < order[j].Method
	})
	for _, route := range order {
		spec := specs[routeKey(route.Method, route.Path)]
		item, ok := doc.Paths[route.Path]
		if !ok {
			item = &PathItem{}
			doc.Paths[route.Path] = item
		}
		item.set(route.Method, operation(r, spec, route))
	}

	doc.Components.Schemas = r.Schemas()
	return doc, nil
}

func routeKey(method, path string) string { return method + " " + path }

func operation(r *Reflector, spec RouteSpec, route Route) *Operation {
	op := &Operation{
		Tags:        []string{spec.Tag},
		Summary:     spec.Summary,
		OperationID: operationID(route.Handler),
		Responses:   map[string]*Response{},
	}

	for _, name := range pathParams(spec.Path) {
		schema := spec.PathParams[name]
		if schema == nil {
			schema = &Schema{Type: "string", Format: "uuid"}
		}
		op.Parameters = append(op.Parameters, Parameter{
			Name: name, In: "path", Required: true, Schema: schema,
		})
	}
	for _, name := range sortedKeys(spec.Query) {
		op.Parameters = append(op.Parameters, Parameter{
			Name: name, In: "query", Schema: r.SchemaOf(reflect.TypeOf(spec.Query[name])),
		})
	}

	switch {
	case spec.Request != nil:
		op.RequestBody = &RequestBody{
			Required: true,
			Content:  jsonContent(r.SchemaOf(reflect.TypeOf(spec.Request))),
		}
	case len(spec.FormFields) > 0 || len(spec.FormFiles) > 0:
		op.RequestBody = formBody(r, spec)
	}

	status := spec.Status
	if status == 0 {
		status = 200
	}
	success := &Response{Description: "Success"}
	switch {
	case spec.ContentType != "":
		success.Content = map[string]MediaType{spec.ContentType: {}}
	case spec.Response != nil:
		schema := r.SchemaOf(reflect.TypeOf(spec.Response))
		if spec.EnvelopeKey != "" {
			schema = &Schema{
				Type:       "object",
				Properties: map[string]*Schema{spec.EnvelopeKey: schema},
				Required:   []string{spec.EnvelopeKey},
			}
		}
		success.Content = jsonContent(schema)
	}
	op.Responses[fmt.Sprint(status)] = success

	errorSchema := r.SchemaOf(reflect.TypeOf(ErrorResponse{}))
	if !spec.Public {
		op.Responses["401"] = &Response{
			Description: "No session, or the session is no longer valid.",
			Content:     jsonContent(errorSchema),
		}
		op.Security = securityFor(spec.Method)
	}
	op.Responses["default"] = &Response{
		Description: "Any error, carrying the shared error envelope.",
		Content:     jsonContent(errorSchema),
	}
	return op
}

// formBody describes a multipart/form-data request, which the importer, the
// revision uploader and the submitter all take.
func formBody(r *Reflector, spec RouteSpec) *RequestBody {
	props := map[string]*Schema{}
	for _, name := range sortedKeys(spec.FormFields) {
		props[name] = r.SchemaOf(reflect.TypeOf(spec.FormFields[name]))
	}
	for _, name := range spec.FormFiles {
		props[name] = &Schema{Type: "string", Format: "binary"}
	}
	required := make([]string, 0, len(props))
	for name := range props {
		required = append(required, name)
	}
	sort.Strings(required)
	return &RequestBody{
		Required: true,
		Content: map[string]MediaType{
			"multipart/form-data": {Schema: &Schema{
				Type: "object", Properties: props, Required: required,
			}},
		},
	}
}

// securityFor mirrors the server: the session cookie always, plus the CSRF
// header on methods that change state.
func securityFor(method string) []SecurityRequirement {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return []SecurityRequirement{{cookieScheme: {}}}
	default:
		return []SecurityRequirement{{cookieScheme: {}, csrfScheme: {}}}
	}
}

// operationID takes the handler name from the router, so ids track renames
// instead of being invented here.
func operationID(handler string) string {
	name := handler
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSuffix(name, "-fm")
	return name
}

// pathParams pulls ":name" segments out of a Gin path template.
func pathParams(path string) []string {
	var out []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, ":") {
			out = append(out, strings.TrimPrefix(seg, ":"))
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// serverRoot locates the module root from this file, so enum sources resolve
// the same way no matter which directory the generator is run from.
func serverRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate the openapi package on disk")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}
