package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/list"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	Fields[FieldTypeMedia] = func() Field {
		return &MediaField{}
	}
}

const FieldTypeMedia = "media"

var (
	_ Field        = (*MediaField)(nil)
	_ MultiValuer  = (*MediaField)(nil)
	_ DriverValuer = (*MediaField)(nil)
)

// MediaField defines a field for referencing items managed through the
// shared _medias system collection.
type MediaField struct {
	Name string `form:"name" json:"name"`
	Id   string `form:"id" json:"id"`

	System bool `form:"system" json:"system"`
	Hidden bool `form:"hidden" json:"hidden"`

	Presentable bool   `form:"presentable" json:"presentable"`
	Help        string `form:"help" json:"help"`

	MaxSelect int `form:"maxSelect" json:"maxSelect"`

	MimeTypes []string `form:"mimeTypes" json:"mimeTypes"`

	AllowFolders bool `form:"allowFolders" json:"allowFolders"`
	Required     bool `form:"required" json:"required"`
}

func (f *MediaField) Type() string {
	return FieldTypeMedia
}

func (f *MediaField) GetId() string {
	return f.Id
}

func (f *MediaField) SetId(id string) {
	f.Id = id
}

func (f *MediaField) GetName() string {
	return f.Name
}

func (f *MediaField) SetName(name string) {
	f.Name = name
}

func (f *MediaField) GetSystem() bool {
	return f.System
}

func (f *MediaField) SetSystem(system bool) {
	f.System = system
}

func (f *MediaField) GetHidden() bool {
	return f.Hidden
}

func (f *MediaField) SetHidden(hidden bool) {
	f.Hidden = hidden
}

func (f *MediaField) IsMultiple() bool {
	return f.MaxSelect > 1
}

func (f *MediaField) ColumnType(app App) string {
	if f.IsMultiple() {
		return "JSON DEFAULT '[]' NOT NULL"
	}

	return "TEXT DEFAULT '' NOT NULL"
}

func (f *MediaField) PrepareValue(record *Record, raw any) (any, error) {
	return f.normalizeValue(raw), nil
}

func (f *MediaField) DriverValue(record *Record) (driver.Value, error) {
	paths := f.toMediaRefSlice(record.GetRaw(f.Name))

	if !f.IsMultiple() {
		if len(paths) > 0 {
			return paths[len(paths)-1], nil
		}
		return "", nil
	}

	return append(types.JSONArray[string]{}, paths...), nil
}

func (f *MediaField) ValidateValue(ctx context.Context, app App, record *Record) error {
	paths := f.toMediaRefSlice(record.GetRaw(f.Name))
	if len(paths) == 0 {
		if f.Required {
			return validation.ErrRequired
		}
		return nil
	}

	maxSelect := max(f.MaxSelect, 1)
	if len(paths) > maxSelect {
		return validation.NewError("validation_too_many_values", "Select no more than {{.maxSelect}}").
			SetParams(map[string]any{"maxSelect": maxSelect})
	}

	originalPaths := map[string]struct{}{}
	for _, p := range f.toMediaRefSlice(record.Original().GetRaw(f.Name)) {
		originalPaths[p] = struct{}{}
	}

	for _, mediaPath := range paths {
		media, err := ResolveMediaRecordReference(app, mediaPath)
		if err != nil {
			if err == sql.ErrNoRows {
				if _, ok := originalPaths[mediaPath]; ok {
					continue
				}

				return validation.NewError("validation_missing_media_path", "Failed to resolve the selected media value.")
			}

			return err
		}

		if media.GetString("kind") == MediaKindFolder {
			if !f.AllowFolders {
				return validation.NewError("validation_invalid_media_kind", "Folders are not allowed for this field.")
			}

			continue
		}

		if len(f.MimeTypes) > 0 && !slices.Contains(f.MimeTypes, media.GetString("mime")) {
			return validation.NewError("validation_invalid_media_type", "The selected media type is not allowed.")
		}
	}

	return nil
}

func (f *MediaField) ValidateSettings(ctx context.Context, app App, collection *Collection) error {
	return validation.ValidateStruct(f,
		validation.Field(&f.Id, validation.By(DefaultFieldIdValidationRule)),
		validation.Field(&f.Name, validation.By(DefaultFieldNameValidationRule)),
		validation.Field(&f.Help, validation.By(DefaultFieldHelpValidationRule)),
		validation.Field(&f.MaxSelect, validation.Min(0), validation.Max(maxSafeJSONInt)),
		validation.Field(&f.MimeTypes, validation.Each(validation.Required, validation.Length(1, 255))),
	)
}

func (f *MediaField) normalizeValue(raw any) any {
	paths := f.toMediaRefSlice(raw)

	if !f.IsMultiple() {
		if len(paths) > 0 {
			return paths[len(paths)-1]
		}

		return ""
	}

	return paths
}

func (f *MediaField) toMediaRefSlice(raw any) []string {
	normalized := make([]string, 0, len(list.ToUniqueStringSlice(raw)))
	seen := map[string]struct{}{}

	for _, mediaPath := range list.ToUniqueStringSlice(raw) {
		mediaPath = normalizeMediaReference(mediaPath)
		if mediaPath == "" || mediaPath == "/" {
			continue
		}

		if _, exists := seen[mediaPath]; exists {
			continue
		}

		seen[mediaPath] = struct{}{}
		normalized = append(normalized, mediaPath)
	}

	return normalized
}

func normalizeMediaReference(value string) string {
	if mediaURL := NormalizeMediaFileURL(value); mediaURL != "" {
		return mediaURL
	}

	return NormalizeMediaPath(value)
}
