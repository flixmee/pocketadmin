package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameMedias = "_medias"
	MediaKindFile        = "file"
	MediaKindFolder      = "folder"
)

var (
	_ Model        = (*Media)(nil)
	_ PreValidator = (*Media)(nil)
	_ RecordProxy  = (*Media)(nil)
)

// Media defines a Record proxy for working with the medias collection.
type Media struct {
	*Record
}

func NewMedia(app App) *Media {
	m := &Media{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameMedias)
	if err != nil {
		c = NewBaseCollection("@__invalid__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *Media) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameMedias {
		return errors.New("missing or invalid media ProxyRecord")
	}

	return nil
}

func (m *Media) ProxyRecord() *Record {
	return m.Record
}

func (m *Media) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Media) Kind() string {
	return m.GetString("kind")
}

func (m *Media) Parent() string {
	return m.GetString("parent")
}

func (m *Media) Name() string {
	return m.GetString("name")
}

func (m *Media) File() string {
	return m.GetString("file")
}

func (m *Media) Mime() string {
	return m.GetString("mime")
}

func (m *Media) Size() int64 {
	return int64(m.GetFloat("size"))
}

func (m *Media) Created() types.DateTime {
	return m.GetDateTime("created")
}

func (m *Media) Updated() types.DateTime {
	return m.GetDateTime("updated")
}

func NormalizeMediaPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	value = strings.ReplaceAll(value, `\`, "/")
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	cleaned := path.Clean(value)
	if cleaned == "." {
		return ""
	}

	return cleaned
}

func ResolveMediaRecordByPath(app App, mediaPath string) (*Record, error) {
	mediaPath = NormalizeMediaPath(mediaPath)
	if mediaPath == "" || mediaPath == "/" {
		return nil, sql.ErrNoRows
	}

	segments := strings.Split(strings.TrimPrefix(mediaPath, "/"), "/")
	parentId := ""
	var current *Record

	for _, segment := range segments {
		records, err := app.FindAllRecords(
			CollectionNameMedias,
			dbx.HashExp{"parent": parentId, "name": segment},
		)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return nil, sql.ErrNoRows
		}

		current = records[0]
		parentId = current.Id
	}

	if current == nil {
		return nil, sql.ErrNoRows
	}

	return current, nil
}

func NormalizeMediaFileURL(value string) string {
	parsed, _, _, _, ok := parseMediaFileURL(value)
	if !ok {
		return ""
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""

	if parsed.Scheme == "" && parsed.Host == "" {
		return parsed.Path
	}

	return parsed.String()
}

func ResolveMediaRecordByURL(app App, value string) (*Record, error) {
	_, collectionRef, recordId, filename, ok := parseMediaFileURL(value)
	if !ok {
		return nil, sql.ErrNoRows
	}

	collection, err := app.FindCollectionByNameOrId(collectionRef)
	if err != nil {
		return nil, sql.ErrNoRows
	}

	if collection.Name != CollectionNameMedias {
		return nil, sql.ErrNoRows
	}

	record, err := app.FindRecordById(collection, recordId)
	if err != nil {
		return nil, err
	}

	if record.GetString("kind") != MediaKindFile || record.GetString("file") != filename {
		return nil, sql.ErrNoRows
	}

	return record, nil
}

func ResolveMediaRecordReference(app App, value string) (*Record, error) {
	if NormalizeMediaFileURL(value) != "" {
		return ResolveMediaRecordByURL(app, value)
	}

	return ResolveMediaRecordByPath(app, value)
}

func BuildMediaPath(app App, record *Record) (string, error) {
	if record == nil || record.Collection().Name != CollectionNameMedias {
		return "", errors.New("missing or invalid media record")
	}

	segments := []string{}
	current := record

	for current != nil {
		name := strings.TrimSpace(current.GetString("name"))
		if name == "" {
			return "", errors.New("media record name cannot be empty")
		}

		segments = append([]string{name}, segments...)

		parentId := current.GetString("parent")
		if parentId == "" {
			break
		}

		parent, err := app.FindRecordById(CollectionNameMedias, parentId)
		if err != nil {
			return "", err
		}

		current = parent
	}

	return "/" + strings.Join(segments, "/"), nil
}

func parseMediaFileURL(value string) (*url.URL, string, string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, "", "", "", false
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return nil, "", "", "", false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "files" {
		return nil, "", "", "", false
	}

	collectionRef, err := url.PathUnescape(parts[2])
	if err != nil || collectionRef == "" {
		return nil, "", "", "", false
	}

	recordId, err := url.PathUnescape(parts[3])
	if err != nil || recordId == "" {
		return nil, "", "", "", false
	}

	filename, err := url.PathUnescape(parts[4])
	if err != nil || filename == "" {
		return nil, "", "", "", false
	}

	parsed.Path = "/api/files/" + url.PathEscape(collectionRef) + "/" + url.PathEscape(recordId) + "/" + url.PathEscape(filename)

	return parsed, collectionRef, recordId, filename, true
}

func (app *BaseApp) registerMediaHooks() {
	app.OnRecordValidate(CollectionNameMedias).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := normalizeMediaRecord(e.Record); err != nil {
				return err
			}

			if err := validateMediaRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 98,
	})

	app.OnRecordCreateExecute(CollectionNameMedias).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := populateMediaFileMeta(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 100,
	})

	app.OnRecordUpdateExecute(CollectionNameMedias).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := populateMediaFileMeta(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 100,
	})
}

func normalizeMediaRecord(record *Record) error {
	if record.GetString("kind") != MediaKindFile {
		return nil
	}

	if strings.TrimSpace(record.GetString("name")) != "" {
		return nil
	}

	if upload, ok := record.GetRaw("file").(*filesystem.File); ok && upload != nil {
		record.Set("name", upload.OriginalName)
		return nil
	}

	uploads := record.GetUnsavedFiles("file")
	if len(uploads) == 0 {
		return nil
	}

	record.Set("name", uploads[len(uploads)-1].OriginalName)

	return nil
}

func validateMediaRecord(app App, record *Record) error {
	kind := record.GetString("kind")
	if err := validation.Validate(kind, validation.Required, validation.In(MediaKindFile, MediaKindFolder)); err != nil {
		return validation.Errors{"kind": err}
	}

	name := strings.TrimSpace(record.GetString("name"))
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return validation.Errors{"name": validation.NewError("validation_invalid_media_name", "Media names cannot contain path separators.")}
	}

	parentId := record.GetString("parent")
	if parentId != "" {
		if record.Id != "" && parentId == record.Id {
			return validation.Errors{"parent": validation.NewError("validation_invalid_parent", "A media item cannot be its own parent.")}
		}

		parent, err := app.FindRecordById(CollectionNameMedias, parentId)
		if err != nil {
			return validation.Errors{"parent": validation.NewError("validation_missing_parent", "The selected parent folder does not exist.")}
		}

		if parent.GetString("kind") != MediaKindFolder {
			return validation.Errors{"parent": validation.NewError("validation_invalid_parent_kind", "The selected parent must be a folder.")}
		}

		if record.Id != "" {
			current := parent
			for current != nil {
				if current.Id == record.Id {
					return validation.Errors{"parent": validation.NewError("validation_invalid_parent_cycle", "A folder cannot be moved inside itself or its descendants.")}
				}

				nextParentId := current.GetString("parent")
				if nextParentId == "" {
					break
				}

				current, err = app.FindRecordById(CollectionNameMedias, nextParentId)
				if err != nil {
					return validation.Errors{"parent": validation.NewError("validation_missing_parent", "The selected parent folder does not exist.")}
				}
			}
		}
	}

	switch kind {
	case MediaKindFolder:
		if mediaFileValue(record) != "" {
			return validation.Errors{"file": validation.NewError("validation_folder_file", "Folders cannot have uploaded files.")}
		}
	case MediaKindFile:
		if mediaFileValue(record) == "" {
			return validation.Errors{"file": validation.NewError("validation_missing_file", "A file upload is required.")}
		}
	}

	return nil
}

func populateMediaFileMeta(app App, record *Record) error {
	if record.GetString("kind") != MediaKindFile {
		record.Set("mime", "")
		record.Set("size", 0)
		return nil
	}

	filename := mediaFileValue(record)
	if filename == "" {
		record.Set("mime", "")
		record.Set("size", 0)
		return nil
	}

	var mime string
	var size int64

	uploads := record.GetUnsavedFiles("file")
	if len(uploads) > 0 {
		size = uploads[len(uploads)-1].Size
		detectedMime, err := detectMediaUploadContentType(uploads[len(uploads)-1])
		if err == nil {
			mime = detectedMime
		}
	}

	if mime == "" || size <= 0 {
		fsys, err := app.NewFilesystem()
		if err != nil {
			return err
		}
		defer fsys.Close()

		attrs, err := fsys.Attributes(record.BaseFilesPath() + "/" + filename)
		if err != nil {
			return fmt.Errorf("failed to load media file metadata: %w", err)
		}

		if mime == "" {
			mime = attrs.ContentType
		}
		if size <= 0 {
			size = attrs.Size
		}
	}

	record.Set("mime", mime)
	record.Set("size", size)

	return nil
}

func mediaFileValue(record *Record) string {
	switch raw := record.GetRaw("file").(type) {
	case string:
		return raw
	case *filesystem.File:
		return raw.Name
	default:
		uploads := record.GetUnsavedFiles("file")
		if len(uploads) > 0 {
			return uploads[len(uploads)-1].Name
		}
	}

	return ""
}

func detectMediaUploadContentType(file *filesystem.File) (string, error) {
	reader, err := file.Reader.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	mt, err := mimetype.DetectReader(reader)
	if err != nil {
		return "", err
	}

	return mt.String(), nil
}
