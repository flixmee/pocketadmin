package core_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func TestMediaFieldBaseMethods(t *testing.T) {
	testFieldBaseMethods(t, core.FieldTypeMedia)
}

func TestMediaFieldColumnType(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name     string
		field    *core.MediaField
		expected string
	}{
		{"single", &core.MediaField{}, "TEXT DEFAULT '' NOT NULL"},
		{"multiple", &core.MediaField{MaxSelect: 2}, "JSON DEFAULT '[]' NOT NULL"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			if got := s.field.ColumnType(app); got != s.expected {
				t.Fatalf("expected %q, got %q", s.expected, got)
			}
		})
	}
}

func TestMediaFieldPrepareValue(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	record := core.NewRecord(core.NewBaseCollection("test"))

	scenarios := []struct {
		raw      any
		field    *core.MediaField
		expected string
	}{
		{nil, &core.MediaField{}, `""`},
		{`docs/hero.png`, &core.MediaField{}, `"/docs/hero.png"`},
		{[]string{"/one.png", "two.png"}, &core.MediaField{}, `"/two.png"`},
		{`https://example.com/api/files/_medias/abc123/hero.png?token=test`, &core.MediaField{}, `"https://example.com/api/files/_medias/abc123/hero.png"`},
		{nil, &core.MediaField{MaxSelect: 2}, `[]`},
		{[]string{`docs\hero.png`, "/docs/hero.png", "/nested/file.pdf"}, &core.MediaField{MaxSelect: 2}, `["/docs/hero.png","/nested/file.pdf"]`},
		{[]string{
			`https://example.com/api/files/_medias/abc123/hero.png?token=test`,
			`https://example.com/api/files/_medias/abc123/hero.png`,
			`https://example.com/api/files/_medias/def456/guide.pdf?download=1`,
		}, &core.MediaField{MaxSelect: 2}, `["https://example.com/api/files/_medias/abc123/hero.png","https://example.com/api/files/_medias/def456/guide.pdf"]`},
	}

	for i, s := range scenarios {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			v, err := s.field.PrepareValue(record, s.raw)
			if err != nil {
				t.Fatal(err)
			}

			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}

			if string(raw) != s.expected {
				t.Fatalf("expected %q, got %q", s.expected, raw)
			}
		})
	}
}

func TestMediaFieldValidateValue(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	paths := seedMediaFieldTestData(t, app)

	record := core.NewRecord(core.NewBaseCollection("articles"))

	scenarios := []struct {
		name        string
		field       *core.MediaField
		value       any
		expectError bool
	}{
		{
			name:        "required empty",
			field:       &core.MediaField{Name: "hero", Required: true},
			value:       "",
			expectError: true,
		},
		{
			name:        "valid file path",
			field:       &core.MediaField{Name: "hero"},
			value:       paths.heroPath,
			expectError: false,
		},
		{
			name:        "valid file url",
			field:       &core.MediaField{Name: "hero"},
			value:       paths.heroURL,
			expectError: false,
		},
		{
			name:        "invalid missing path",
			field:       &core.MediaField{Name: "hero"},
			value:       "/missing/file.png",
			expectError: true,
		},
		{
			name:        "folder disallowed",
			field:       &core.MediaField{Name: "hero"},
			value:       paths.folderPath,
			expectError: true,
		},
		{
			name:        "folder allowed",
			field:       &core.MediaField{Name: "hero", AllowFolders: true},
			value:       paths.folderPath,
			expectError: false,
		},
		{
			name:        "mime restricted",
			field:       &core.MediaField{Name: "hero", MimeTypes: []string{"application/pdf"}},
			value:       paths.heroPath,
			expectError: true,
		},
		{
			name:        "multiple max select",
			field:       &core.MediaField{Name: "hero", MaxSelect: 2},
			value:       []string{paths.heroPath, paths.docPath, paths.folderPath},
			expectError: true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			record.SetRaw(s.field.Name, s.value)

			err := s.field.ValidateValue(context.Background(), app, record)
			if (err != nil) != s.expectError {
				t.Fatalf("expected error=%v, got %v", s.expectError, err)
			}
		})
	}
}

func TestMediaFieldValidateSettings(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")

	scenarios := []struct {
		name        string
		field       *core.MediaField
		expectError bool
	}{
		{
			name:        "valid",
			field:       &core.MediaField{Id: "abc123", Name: "hero", MimeTypes: []string{"image/png"}},
			expectError: false,
		},
		{
			name:        "missing name",
			field:       &core.MediaField{Id: "abc123"},
			expectError: true,
		},
		{
			name:        "invalid mime",
			field:       &core.MediaField{Id: "abc123", Name: "hero", MimeTypes: []string{""}},
			expectError: true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			errs, _ := s.field.ValidateSettings(context.Background(), app, collection).(validation.Errors)
			hasErr := len(errs) > 0
			if hasErr != s.expectError {
				t.Fatalf("expected error=%v, got %v", s.expectError, errs)
			}
		})
	}

	testDefaultFieldIdValidation(t, core.FieldTypeMedia)
	testDefaultFieldNameValidation(t, core.FieldTypeMedia)
	testDefaultFieldHelpValidation[core.MediaField](t)
}

func TestMediaPathHelpers(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	paths := seedMediaFieldTestData(t, app)

	if got := core.NormalizeMediaPath(` docs\heroes\cover.png `); got != "/docs/heroes/cover.png" {
		t.Fatalf("unexpected normalized path %q", got)
	}

	if got := core.NormalizeMediaFileURL(paths.heroURL + "?token=test"); got != paths.heroURL {
		t.Fatalf("unexpected normalized url %q", got)
	}

	record, err := core.ResolveMediaRecordByPath(app, paths.heroPath)
	if err != nil {
		t.Fatal(err)
	}

	if record.Id != paths.heroId {
		t.Fatalf("expected hero id %q, got %q", paths.heroId, record.Id)
	}

	built, err := core.BuildMediaPath(app, record)
	if err != nil {
		t.Fatal(err)
	}

	if built != paths.heroPath {
		t.Fatalf("expected built path %q, got %q", paths.heroPath, built)
	}

	record, err = core.ResolveMediaRecordReference(app, paths.heroURL)
	if err != nil {
		t.Fatal(err)
	}

	if record.Id != paths.heroId {
		t.Fatalf("expected hero id %q from url, got %q", paths.heroId, record.Id)
	}
}

type mediaFieldTestPaths struct {
	folderPath string
	heroPath   string
	heroId     string
	heroURL    string
	docPath    string
}

func seedMediaFieldTestData(t *testing.T, app core.App) mediaFieldTestPaths {
	t.Helper()

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		t.Fatal(err)
	}

	rootFolder := core.NewRecord(collection)
	rootFolder.Set("name", "docs")
	rootFolder.Set("kind", core.MediaKindFolder)
	if err := app.Save(rootFolder); err != nil && !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatal(err)
	}
	if rootFolder.Id == "" {
		rootFolder, err = app.FindFirstRecordByFilter(collection, `parent="" && name="docs"`)
		if err != nil {
			t.Fatal(err)
		}
	}

	heroUpload, err := filesystem.NewFileFromBytes([]byte("hero"), "hero.png")
	if err != nil {
		t.Fatal(err)
	}

	hero := core.NewRecord(collection)
	hero.Set("kind", core.MediaKindFile)
	hero.Set("parent", rootFolder.Id)
	hero.Set("file", heroUpload)
	if err := app.Save(hero); err != nil {
		t.Fatal(err)
	}

	docUpload, err := filesystem.NewFileFromBytes([]byte("%PDF"), "guide.pdf")
	if err != nil {
		t.Fatal(err)
	}

	doc := core.NewRecord(collection)
	doc.Set("kind", core.MediaKindFile)
	doc.Set("parent", rootFolder.Id)
	doc.Set("file", docUpload)
	if err := app.Save(doc); err != nil {
		t.Fatal(err)
	}

	return mediaFieldTestPaths{
		folderPath: "/docs",
		heroPath:   "/docs/hero.png",
		heroId:     hero.Id,
		heroURL:    "https://example.com/api/files/" + collection.Id + "/" + hero.Id + "/" + hero.GetString("file"),
		docPath:    "/docs/guide.pdf",
	}
}
