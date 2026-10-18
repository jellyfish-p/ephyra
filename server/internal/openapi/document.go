package openapi

// Document is the subset of OpenAPI 3.0.3 this project emits. It is
// deliberately not a complete model of the spec: fields that would always be
// empty here are left out rather than carried around unused.
type Document struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Servers    []Server             `json:"servers,omitempty"`
	Tags       []Tag                `json:"tags,omitempty"`
	Paths      map[string]*PathItem `json:"paths"`
	Components Components           `json:"components"`
}

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Components struct {
	Schemas         map[string]*Schema         `json:"schemas"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme describes one way a caller authenticates. The API uses a
// session cookie plus a CSRF header, so both are declared and unsafe operations
// require the pair.
type SecurityScheme struct {
	Type        string `json:"type"`
	In          string `json:"in"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operations lists the methods this path serves, upper-cased. Gin can only
// register the five handled here, so the set is closed.
func (p *PathItem) Operations() map[string]*Operation {
	out := map[string]*Operation{}
	for method, op := range map[string]*Operation{
		"GET": p.Get, "POST": p.Post, "PATCH": p.Patch, "PUT": p.Put, "DELETE": p.Delete,
	} {
		if op != nil {
			out[method] = op
		}
	}
	return out
}

func (p *PathItem) set(method string, op *Operation) {
	switch method {
	case "GET":
		p.Get = op
	case "POST":
		p.Post = op
	case "PATCH":
		p.Patch = op
	case "PUT":
		p.Put = op
	case "DELETE":
		p.Delete = op
	}
}

type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]*Response  `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type RequestBody struct {
	Required    bool                 `json:"required,omitempty"`
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// SecurityRequirement names the schemes a request must satisfy.
type SecurityRequirement map[string][]string

func jsonContent(s *Schema) map[string]MediaType {
	return map[string]MediaType{"application/json": {Schema: s}}
}
