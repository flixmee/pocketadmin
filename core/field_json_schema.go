package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// MaxJSONSchemaSize is the maximum allowed JSON Schema size in bytes.
const MaxJSONSchemaSize = 50 * 1024 // 50KB

// MaxJSONSchemaDepth is the maximum nesting depth allowed for JSON schemas.
const MaxJSONSchemaDepth = 10

// jsonSchemaCache stores compiled JSON schemas keyed by "collectionId:fieldName".
var jsonSchemaCache sync.Map

// jsonSchemaCacheEntry holds a compiled schema alongside the source string
// so we can detect when the schema text has changed without invalidation.
type jsonSchemaCacheEntry struct {
	source string
	schema *jsonschema.Schema
}

// CompileJSONSchema compiles a JSON Schema string (draft-07) and returns
// the compiled schema instance.
func CompileJSONSchema(schemaStr string) (*jsonschema.Schema, error) {
	var doc any
	if err := json.Unmarshal([]byte(schemaStr), &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)

	if err := compiler.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}

	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile JSON Schema: %w", err)
	}

	return compiled, nil
}

// GetOrCompileJSONSchema returns a compiled schema from cache or compiles
// and caches a new one. The cache key is "collectionId:fieldName".
func GetOrCompileJSONSchema(collectionId, fieldName, schemaStr string) (*jsonschema.Schema, error) {
	key := collectionId + ":" + fieldName

	if cached, ok := jsonSchemaCache.Load(key); ok {
		entry := cached.(*jsonSchemaCacheEntry)
		if entry.source == schemaStr {
			return entry.schema, nil
		}
	}

	compiled, err := CompileJSONSchema(schemaStr)
	if err != nil {
		return nil, err
	}

	jsonSchemaCache.Store(key, &jsonSchemaCacheEntry{
		source: schemaStr,
		schema: compiled,
	})

	return compiled, nil
}

// InvalidateJSONSchemaCache removes the cached compiled schema for
// a given collection + field combination.
func InvalidateJSONSchemaCache(collectionId, fieldName string) {
	key := collectionId + ":" + fieldName
	jsonSchemaCache.Delete(key)
}

// CheckJSONSchemaDepth checks whether the JSON schema string exceeds
// the maximum allowed nesting depth. Returns an error if depth > maxDepth.
func CheckJSONSchemaDepth(schemaStr string, maxDepth int) error {
	var raw any
	if err := json.Unmarshal([]byte(schemaStr), &raw); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	depth := measureDepth(raw)
	if depth > maxDepth {
		return fmt.Errorf("schema nesting depth %d exceeds maximum allowed depth of %d", depth, maxDepth)
	}

	return nil
}

// measureDepth returns the maximum nesting depth of a JSON value.
func measureDepth(v any) int {
	switch val := v.(type) {
	case map[string]any:
		max := 0
		for _, child := range val {
			d := measureDepth(child)
			if d > max {
				max = d
			}
		}
		return max + 1
	case []any:
		max := 0
		for _, child := range val {
			d := measureDepth(child)
			if d > max {
				max = d
			}
		}
		return max + 1
	default:
		return 1
	}
}

// ExtractFirstValidationError walks the jsonschema.ValidationError tree
// and returns the first leaf error with its instance path as a
// dot-separated string (e.g., "address.zip").
func ExtractFirstValidationError(err *jsonschema.ValidationError) (instancePath string, message string) {
	// Walk to the first leaf error (deepest cause).
	current := err
	for len(current.Causes) > 0 {
		current = current.Causes[0]
	}

	// Build instance path from InstanceLocation slice.
	if len(current.InstanceLocation) > 0 {
		instancePath = strings.Join(current.InstanceLocation, ".")
	}

	// Use the error message from ErrorKind.
	if current.ErrorKind != nil {
		message = fmt.Sprintf("%v", current.ErrorKind)
	} else {
		message = current.Error()
	}

	return instancePath, message
}
