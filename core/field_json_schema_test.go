package core

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCompileJSONSchema(t *testing.T) {
	scenarios := []struct {
		name      string
		schema    string
		expectErr bool
	}{
		{
			"valid object schema",
			`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			false,
		},
		{
			"valid string schema",
			`{"type":"string","minLength":1,"maxLength":100}`,
			false,
		},
		{
			"valid number schema",
			`{"type":"number","minimum":0,"maximum":100}`,
			false,
		},
		{
			"valid array schema",
			`{"type":"array","items":{"type":"string"}}`,
			false,
		},
		{
			"valid boolean schema",
			`{"type":"boolean"}`,
			false,
		},
		{
			"valid schema with $schema keyword",
			`{"$schema":"http://json-schema.org/draft-07/schema#","type":"string"}`,
			false,
		},
		{
			"invalid JSON",
			`{"type":`,
			true,
		},
		{
			"invalid type value",
			`{"type":"notavalidtype"}`,
			true,
		},
		{
			"empty string",
			``,
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			_, err := CompileJSONSchema(s.schema)
			hasErr := err != nil
			if hasErr != s.expectErr {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectErr, hasErr, err)
			}
		})
	}
}

func TestCheckJSONSchemaDepth(t *testing.T) {
	scenarios := []struct {
		name      string
		schema    string
		maxDepth  int
		expectErr bool
	}{
		{
			"flat schema within limit",
			`{"type":"string"}`,
			10,
			false,
		},
		{
			"nested schema within limit",
			`{"type":"object","properties":{"a":{"type":"object","properties":{"b":{"type":"string"}}}}}`,
			10,
			false,
		},
		{
			"deeply nested schema exceeding limit",
			`{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"v"}}}}}}}}}}}`,
			10,
			true,
		},
		{
			"exactly at limit",
			`{"a":"b"}`,
			2,
			false,
		},
		{
			"one level over limit",
			`{"a":{"b":"c"}}`,
			2,
			true,
		},
		{
			"invalid JSON",
			`{invalid`,
			10,
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := CheckJSONSchemaDepth(s.schema, s.maxDepth)
			hasErr := err != nil
			if hasErr != s.expectErr {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectErr, hasErr, err)
			}
		})
	}
}

func TestExtractFirstValidationError(t *testing.T) {
	// Compile a schema and validate against it to get real ValidationErrors.
	schema, err := CompileJSONSchema(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0},
			"address": {
				"type": "object",
				"properties": {
					"zip": {"type": "string", "pattern": "^[0-9]{5}$"}
				},
				"required": ["zip"]
			}
		},
		"required": ["name"]
	}`)
	if err != nil {
		t.Fatalf("Failed to compile test schema: %v", err)
	}

	scenarios := []struct {
		name         string
		value        any
		expectPath   string
		expectErrMsg bool
	}{
		{
			"missing required field",
			map[string]any{"other": "value"},
			"",
			true,
		},
		{
			"wrong type for property",
			map[string]any{"name": 123},
			"name",
			true,
		},
		{
			"nested validation error",
			map[string]any{"name": "test", "address": map[string]any{"zip": "abc"}},
			"address.zip",
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := schema.Validate(s.value)
			if err == nil {
				t.Fatal("Expected validation error, got nil")
			}

			valErr, ok := err.(*jsonschema.ValidationError)
			if !ok {
				t.Fatalf("Expected *jsonschema.ValidationError, got %T", err)
			}

			path, msg := ExtractFirstValidationError(valErr)

			if s.expectPath != "" && path != s.expectPath {
				t.Fatalf("Expected path %q, got %q", s.expectPath, path)
			}

			if s.expectErrMsg && msg == "" {
				t.Fatal("Expected non-empty error message, got empty")
			}
		})
	}
}

func TestGetOrCompileJSONSchemaCache(t *testing.T) {
	schemaStr := `{"type":"string"}`

	// First call should compile and cache.
	s1, err := GetOrCompileJSONSchema("col1", "field1", schemaStr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Second call with same inputs should return cached.
	s2, err := GetOrCompileJSONSchema("col1", "field1", schemaStr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if s1 != s2 {
		t.Fatal("Expected same cached schema instance")
	}

	// Different schema string should recompile.
	s3, err := GetOrCompileJSONSchema("col1", "field1", `{"type":"number"}`)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if s3 == s1 {
		t.Fatal("Expected different schema instance after schema change")
	}

	// Invalidation should cause recompilation.
	InvalidateJSONSchemaCache("col1", "field1")
	s4, err := GetOrCompileJSONSchema("col1", "field1", `{"type":"number"}`)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if s4 == s3 {
		t.Fatal("Expected different schema instance after invalidation")
	}
}

func TestMeasureDepth(t *testing.T) {
	scenarios := []struct {
		name     string
		value    any
		expected int
	}{
		{"string", "hello", 1},
		{"number", 42.0, 1},
		{"bool", true, 1},
		{"nil", nil, 1},
		{"flat object", map[string]any{"a": "b"}, 2},
		{"nested object", map[string]any{"a": map[string]any{"b": "c"}}, 3},
		{"flat array", []any{1, 2, 3}, 2},
		{"nested array", []any{[]any{1}}, 3},
		{"mixed", map[string]any{"a": []any{map[string]any{"b": "c"}}}, 4},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			result := measureDepth(s.value)
			if result != s.expected {
				t.Fatalf("Expected depth %d, got %d", s.expected, result)
			}
		})
	}
}

func TestMaxJSONSchemaSize(t *testing.T) {
	// Schema under the limit should be fine.
	smallSchema := `{"type":"string"}`
	if len(smallSchema) > MaxJSONSchemaSize {
		t.Fatalf("Test schema should be under the limit")
	}

	// Schema over the limit.
	largeSchema := `{"type":"string","description":"` + strings.Repeat("x", MaxJSONSchemaSize+1) + `"}`
	if len(largeSchema) <= MaxJSONSchemaSize {
		t.Fatalf("Test schema should be over the limit, got %d bytes", len(largeSchema))
	}
}
