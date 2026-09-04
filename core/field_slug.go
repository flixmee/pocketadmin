package core

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"unicode"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core/validators"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/spf13/cast"
)

func init() {
	Fields[FieldTypeSlug] = func() Field {
		return &SlugField{}
	}
}

const FieldTypeSlug = "slug"

var slugValueRegex = regexp.MustCompile(`^[\p{L}\p{N}]+(?:-[\p{L}\p{N}]+)*$`)

var _ Field = (*SlugField)(nil)
var _ RecordInterceptor = (*SlugField)(nil)

// SlugField defines a field for storing a URL-friendly slug string value.
type SlugField struct {
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

	// Min specifies the minimum required string characters.
	//
	// if zero value, no min limit is applied.
	Min int `form:"min" json:"min"`

	// Max specifies the maximum allowed string characters.
	//
	// If zero, a default limit of 5000 is applied.
	Max int `form:"max" json:"max"`

	// AttachedField optionally references another field name whose value will be used as the slug source.
	AttachedField string `form:"attachedField" json:"attachedField"`

	// Required will require the field value to be non-empty string.
	Required bool `form:"required" json:"required"`
}

// Type implements [Field.Type] interface method.
func (f *SlugField) Type() string {
	return FieldTypeSlug
}

// GetId implements [Field.GetId] interface method.
func (f *SlugField) GetId() string {
	return f.Id
}

// SetId implements [Field.SetId] interface method.
func (f *SlugField) SetId(id string) {
	f.Id = id
}

// GetName implements [Field.GetName] interface method.
func (f *SlugField) GetName() string {
	return f.Name
}

// SetName implements [Field.SetName] interface method.
func (f *SlugField) SetName(name string) {
	f.Name = name
}

// GetSystem implements [Field.GetSystem] interface method.
func (f *SlugField) GetSystem() bool {
	return f.System
}

// SetSystem implements [Field.SetSystem] interface method.
func (f *SlugField) SetSystem(system bool) {
	f.System = system
}

// GetHidden implements [Field.GetHidden] interface method.
func (f *SlugField) GetHidden() bool {
	return f.Hidden
}

// SetHidden implements [Field.SetHidden] interface method.
func (f *SlugField) SetHidden(hidden bool) {
	f.Hidden = hidden
}

// ColumnType implements [Field.ColumnType] interface method.
func (f *SlugField) ColumnType(app App) string {
	return "TEXT DEFAULT '' NOT NULL"
}

// PrepareValue implements [Field.PrepareValue] interface method.
func (f *SlugField) PrepareValue(record *Record, raw any) (any, error) {
	return normalizeSlug(cast.ToString(raw)), nil
}

// ValidateValue implements [Field.ValidateValue] interface method.
func (f *SlugField) ValidateValue(ctx context.Context, app App, record *Record) error {
	val, ok := record.GetRaw(f.Name).(string)
	if !ok {
		return validators.ErrUnsupportedValueType
	}

	if f.Required {
		if err := validation.Required.Validate(val); err != nil {
			return err
		}
	}

	if val == "" {
		return nil
	}

	normalized := normalizeSlug(val)
	if normalized == "" {
		return validation.NewError("validation_invalid_format", "Invalid value format.")
	}

	length := len([]rune(normalized))
	if f.Min > 0 && length < f.Min {
		return validation.NewError("validation_min_text_constraint", "Must be at least {{.min}} character(s).").
			SetParams(map[string]any{"min": f.Min})
	}

	max := f.Max
	if max == 0 {
		max = 5000
	}

	if max > 0 && length > max {
		return validation.NewError("validation_max_text_constraint", "Must be no more than {{.max}} character(s).").
			SetParams(map[string]any{"max": max})
	}

	if !slugValueRegex.MatchString(normalized) {
		return validation.NewError("validation_invalid_format", "Invalid value format.")
	}

	return nil
}

// ValidateSettings implements [Field.ValidateSettings] interface method.
func (f *SlugField) ValidateSettings(ctx context.Context, app App, collection *Collection) error {
	return validation.ValidateStruct(f,
		validation.Field(&f.Id, validation.By(DefaultFieldIdValidationRule)),
		validation.Field(&f.Name, validation.By(DefaultFieldNameValidationRule)),
		validation.Field(&f.Help, validation.By(DefaultFieldHelpValidationRule)),
		validation.Field(&f.Min, validation.Min(0), validation.Max(maxSafeJSONInt)),
		validation.Field(&f.Max, validation.Min(f.Min), validation.Max(maxSafeJSONInt)),
		validation.Field(&f.AttachedField,
			validation.When(f.AttachedField != "", validation.By(DefaultFieldNameValidationRule)),
			validation.By(f.checkAttachedField(collection)),
		),
	)
}

// Intercept implements the [RecordInterceptor] interface.
func (f *SlugField) Intercept(
	ctx context.Context,
	app App,
	record *Record,
	actionName string,
	actionFunc func() error,
) error {
	switch actionName {
	case InterceptorActionValidate, InterceptorActionCreate, InterceptorActionUpdate, InterceptorActionCreateExecute, InterceptorActionUpdateExecute:
		f.syncAttachedValue(app, record)
	}

	return actionFunc()
}

func (f *SlugField) checkAttachedField(collection *Collection) validation.RuleFunc {
	return func(value any) error {
		name, _ := value.(string)
		if name == "" {
			return nil
		}

		if name == f.Name {
			return validation.NewError("validation_invalid_attached_field", "The attached field cannot reference itself.")
		}

		if collection != nil && collection.Fields.GetByName(name) == nil {
			return validation.NewError("validation_invalid_attached_field", "The attached field does not exist.")
		}

		return nil
	}
}

func (f *SlugField) syncAttachedValue(app App, record *Record) {
	current := record.GetString(f.Name)
	normalizedCurrent := normalizeSlug(current)

	// when no attached field is set -> autogenerate/value normalize
	if f.AttachedField == "" {
		if current == "" {
			// generate unique random string using Min or fallback to 8
			length := 8
			if f.Min > 0 {
				length = f.Min
			}

			// try a few attempts to find a unique candidate
			var candidate string
			for i := 0; i < 10; i++ {
				candidate = strings.ToLower(security.RandomString(length))

				// check uniqueness in the collection for this field
				_, err := app.FindFirstRecordByFilter(record.Collection().Id, f.Name+"={:val}", dbx.Params{"val": candidate})
				if err == sql.ErrNoRows {
					record.SetRaw(f.Name, candidate)
					return
				}
				if err != nil {
					// on unexpected error, fallback to next attempt
					break
				}
			}

			// fallback: set something even if not proven unique
			record.SetRaw(f.Name, strings.ToLower(security.RandomString(length)))
			return
		}

		if current != normalizedCurrent {
			record.SetRaw(f.Name, normalizedCurrent)
		}
		return
	}

	sourceField := record.Collection().Fields.GetByName(f.AttachedField)
	if sourceField == nil {
		return
	}

	source := record.GetString(sourceField.GetName())
	originalSource := record.Original().GetString(sourceField.GetName())
	originalSlug := normalizeSlug(originalSource)

	if source == "" {
		if current == "" || current == originalSlug {
			record.SetRaw(f.Name, "")
		} else if current != normalizedCurrent {
			record.SetRaw(f.Name, normalizedCurrent)
		}
		return
	}

	candidate := normalizeSlug(source)
	if current == "" || current == originalSlug {
		record.SetRaw(f.Name, candidate)
		return
	}

	if current != normalizedCurrent {
		record.SetRaw(f.Name, normalizedCurrent)
	}
}

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastWasSeparator := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			builder.WriteRune(r)
			lastWasSeparator = false
		default:
			if builder.Len() > 0 && !lastWasSeparator {
				builder.WriteByte('-')
				lastWasSeparator = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}
