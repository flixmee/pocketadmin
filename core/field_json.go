package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/ozzo-validation/v4/is"
	"github.com/pocketbase/pocketbase/core/validators"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func init() {
	Fields[FieldTypeJSON] = func() Field {
		return &JSONField{}
	}
}

const FieldTypeJSON = "json"

const DefaultJSONFieldMaxSize int64 = 1 << 20

var (
	_ Field                 = (*JSONField)(nil)
	_ MaxBodySizeCalculator = (*JSONField)(nil)
)

// JSONField defines "json" type field for storing any serialized JSON value.
//
// The respective zero record field value is the zero [types.JSONRaw].
type JSONField struct {
	// Name (required) is the unique name of the field.
	Name string `form:"name" json:"name"`

	// Id is the unique stable field identifier.
	//
	// It is automatically generated from the name when adding to a collection FieldsList.
	Id string `form:"id" json:"id"`

	// System prevents the renaming and removal of the field.
	System bool `form:"system" json:"system"`

	// Hidden hides the field from the API response.
	Hidden bool `form:"hidden" json:"hidden"`

	// ---

	// Presentable hints the Dashboard UI to use the underlying
	// field record value in the relation preview label.
	Presentable bool `form:"presentable" json:"presentable"`

	// Help is an extra text explaining what the field is about.
	// It is usually shown in Dashboard UI under the field input.
	Help string `form:"help" json:"help"`

	// MaxSize specifies the maximum size of the allowed field value (in bytes and up to 2^53-1).
	//
	// If zero, a default limit of 1MB is applied.
	MaxSize int64 `form:"maxSize" json:"maxSize"`

	// Required will require the field value to be non-empty JSON value
	// (aka. not "null", `""`, "[]", "{}").
	Required bool `form:"required" json:"required"`

	// JsonSchema is an optional JSON Schema (draft-07) definition string.
	// When set, non-null and non-empty record values are validated against it.
	JsonSchema string `form:"jsonSchema" json:"jsonSchema"`
}

// Type implements [Field.Type] interface method.
func (f *JSONField) Type() string {
	return FieldTypeJSON
}

// GetId implements [Field.GetId] interface method.
func (f *JSONField) GetId() string {
	return f.Id
}

// SetId implements [Field.SetId] interface method.
func (f *JSONField) SetId(id string) {
	f.Id = id
}

// GetName implements [Field.GetName] interface method.
func (f *JSONField) GetName() string {
	return f.Name
}

// SetName implements [Field.SetName] interface method.
func (f *JSONField) SetName(name string) {
	f.Name = name
}

// GetSystem implements [Field.GetSystem] interface method.
func (f *JSONField) GetSystem() bool {
	return f.System
}

// SetSystem implements [Field.SetSystem] interface method.
func (f *JSONField) SetSystem(system bool) {
	f.System = system
}

// GetHidden implements [Field.GetHidden] interface method.
func (f *JSONField) GetHidden() bool {
	return f.Hidden
}

// SetHidden implements [Field.SetHidden] interface method.
func (f *JSONField) SetHidden(hidden bool) {
	f.Hidden = hidden
}

// ColumnType implements [Field.ColumnType] interface method.
func (f *JSONField) ColumnType(app App) string {
	return "JSON DEFAULT NULL"
}

// PrepareValue implements [Field.PrepareValue] interface method.
func (f *JSONField) PrepareValue(record *Record, raw any) (any, error) {
	if str, ok := raw.(string); ok {
		// in order to support seamlessly both json and multipart/form-data requests,
		// the following normalization rules are applied for plain string values:
		// - "true" is converted to the json `true`
		// - "false" is converted to the json `false`
		// - "null" is converted to the json `null`
		// - "[1,2,3]" is converted to the json `[1,2,3]`
		// - "{\"a\":1,\"b\":2}" is converted to the json `{"a":1,"b":2}`
		// - numeric strings are converted to json number
		// - double quoted strings are left as they are (aka. without normalizations)
		// - any other string (empty string too) is double quoted
		if str == "" {
			raw = strconv.Quote(str)
		} else if str == "null" || str == "true" || str == "false" {
			raw = str
		} else if ((str[0] >= '0' && str[0] <= '9') ||
			str[0] == '-' ||
			str[0] == '"' ||
			str[0] == '[' ||
			str[0] == '{') &&
			is.JSON.Validate(str) == nil {
			raw = str
		} else {
			raw = strconv.Quote(str)
		}
	}

	return types.ParseJSONRaw(raw)
}

var emptyJSONValues = []string{
	"null", `""`, "[]", "{}", "",
}

// ValidateValue implements [Field.ValidateValue] interface method.
func (f *JSONField) ValidateValue(ctx context.Context, app App, record *Record) error {
	raw, ok := record.GetRaw(f.Name).(types.JSONRaw)
	if !ok {
		return validators.ErrUnsupportedValueType
	}

	maxSize := f.CalculateMaxBodySize()

	if int64(len(raw)) > maxSize {
		return validation.NewError(
			"validation_json_size_limit",
			"The maximum allowed JSON size is {{.maxSize}} bytes",
		).SetParams(map[string]any{"maxSize": maxSize})
	}

	if is.JSON.Validate(raw) != nil {
		return validation.NewError("validation_invalid_json", "Must be a valid json value")
	}

	rawStr := strings.TrimSpace(raw.String())

	if f.Required && slices.Contains(emptyJSONValues, rawStr) {
		return validation.ErrRequired
	}

	// JSON Schema validation (skipped for null/empty values).
	if f.JsonSchema != "" && !slices.Contains(emptyJSONValues, rawStr) {
		collectionId := ""
		if record.Collection() != nil {
			collectionId = record.Collection().Id
		}

		compiledSchema, err := GetOrCompileJSONSchema(collectionId, f.Name, f.JsonSchema)
		if err != nil {
			return validation.NewError(
				"validation_json_schema_compile",
				"Failed to compile JSON Schema: {{.details}}",
			).SetParams(map[string]any{"details": err.Error()})
		}

		var value any
		if err := json.Unmarshal([]byte(rawStr), &value); err != nil {
			return validation.NewError("validation_invalid_json", "Must be a valid json value")
		}

		if err := compiledSchema.Validate(value); err != nil {
			if valErr, ok := err.(*jsonschema.ValidationError); ok {
				instancePath, msg := ExtractFirstValidationError(valErr)
				errorMsg := msg
				if instancePath != "" {
					errorMsg = fmt.Sprintf("%s: %s", instancePath, msg)
				}
				return validation.NewError(
					"validation_json_schema",
					"JSON schema validation failed: {{.details}}",
				).SetParams(map[string]any{"details": errorMsg})
			}
			return validation.NewError(
				"validation_json_schema",
				"JSON schema validation failed: {{.details}}",
			).SetParams(map[string]any{"details": err.Error()})
		}
	}

	return nil
}

// ValidateSettings implements [Field.ValidateSettings] interface method.
func (f *JSONField) ValidateSettings(ctx context.Context, app App, collection *Collection) error {
	// Invalidate cached schema on every settings save so recompilation picks up changes.
	if collection != nil {
		InvalidateJSONSchemaCache(collection.Id, f.Name)
	}

	return validation.ValidateStruct(f,
		validation.Field(&f.Id, validation.By(DefaultFieldIdValidationRule)),
		validation.Field(&f.Name, validation.By(DefaultFieldNameValidationRule)),
		validation.Field(&f.Help, validation.By(DefaultFieldHelpValidationRule)),
		validation.Field(&f.MaxSize, validation.Min(0), validation.Max(maxSafeJSONInt)),
		validation.Field(&f.JsonSchema, validation.By(f.validateJSONSchema)),
	)
}

// validateJSONSchema validates the JsonSchema field option value.
func (f *JSONField) validateJSONSchema(value any) error {
	v, _ := value.(string)
	if v == "" {
		return nil
	}

	// Check size limit.
	if len(v) > MaxJSONSchemaSize {
		return validation.NewError(
			"validation_json_schema_too_large",
			"The JSON Schema exceeds the maximum allowed size of {{.maxSize}} bytes",
		).SetParams(map[string]any{"maxSize": MaxJSONSchemaSize})
	}

	// Check nesting depth.
	if err := CheckJSONSchemaDepth(v, MaxJSONSchemaDepth); err != nil {
		return validation.NewError(
			"validation_json_schema_too_deep",
			"The JSON Schema nesting depth exceeds the maximum of {{.maxDepth}} levels",
		).SetParams(map[string]any{"maxDepth": MaxJSONSchemaDepth})
	}

	// Try to compile the schema to verify structural validity.
	if _, err := CompileJSONSchema(v); err != nil {
		return validation.NewError(
			"validation_json_schema_invalid",
			"Invalid JSON Schema: {{.details}}",
		).SetParams(map[string]any{"details": err.Error()})
	}

	return nil
}

// CalculateMaxBodySize implements the [MaxBodySizeCalculator] interface.
func (f *JSONField) CalculateMaxBodySize() int64 {
	if f.MaxSize <= 0 {
		return DefaultJSONFieldMaxSize
	}

	return f.MaxSize
}
