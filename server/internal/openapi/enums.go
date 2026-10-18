package openapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// StringConstants returns the values of every constant declared with the named
// type in a Go source file, in source order.
//
// Enums are read from the source rather than listed by hand so that adding a
// verdict or a role updates the document. A hand-kept list would be a second
// copy of the API, which is the thing this generator exists to remove.
func StringConstants(filename, typeName string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}
	var values []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		// A spec without its own type continues the previous one, so the type
		// is tracked across the block rather than read per spec.
		lastType := ""
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Type != nil {
				lastType = ""
				if id, ok := vs.Type.(*ast.Ident); ok {
					lastType = id.Name
				}
			}
			if lastType != typeName {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", filename, err)
				}
				values = append(values, s)
			}
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s declares no %s constants", filename, typeName)
	}
	return values, nil
}
