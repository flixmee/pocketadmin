package core_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestI18nLocaleAndRecordLifecycle(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	defaultLocale := app.DefaultLocaleCode()
	if defaultLocale != core.DefaultLocaleCode {
		t.Fatalf("Expected default locale %q, got %q", core.DefaultLocaleCode, defaultLocale)
	}

	vi := core.NewLocale(app)
	vi.SetCode("vi")
	vi.SetName("Vietnamese")
	vi.SetEnabled(true)
	if err := app.Save(vi); err != nil {
		t.Fatalf("Failed to create vi locale: %v", err)
	}

	collection := core.NewBaseCollection("i18n_posts")
	collection.Fields.Add(&core.TextField{Name: "title"})
	collection.I18n.Enabled = true
	collection.I18n.DefaultLocale = core.DefaultLocaleCode
	collection.I18n.LocalizedFields = []string{"title"}
	if err := app.Save(collection); err != nil {
		t.Fatalf("Failed to create localized collection: %v", err)
	}

	if collection.Fields.GetByName(core.FieldNameI18nGroupId) == nil {
		t.Fatalf("Expected i18n group system field")
	}
	if collection.GetIndex("idx_i18n_posts_i18n_unique") == "" {
		t.Fatalf("Expected i18n unique index")
	}

	source := core.NewRecord(collection)
	source.Set("title", "Hello")
	if err := app.Save(source); err != nil {
		t.Fatalf("Failed to create source record: %v", err)
	}
	if source.GetString(core.FieldNameLocale) != core.DefaultLocaleCode {
		t.Fatalf("Expected source locale %q, got %q", core.DefaultLocaleCode, source.GetString(core.FieldNameLocale))
	}
	if source.GetString(core.FieldNameI18nGroupId) == "" {
		t.Fatalf("Expected source i18n group id")
	}
	if !source.GetBool(core.FieldNameIsSource) {
		t.Fatalf("Expected source marker")
	}

	translation := core.NewRecord(collection)
	translation.Set("title", "Xin chao")
	translation.Set(core.FieldNameI18nGroupId, source.GetString(core.FieldNameI18nGroupId))
	translation.Set(core.FieldNameLocale, "vi")
	if err := app.Save(translation); err != nil {
		t.Fatalf("Failed to create translation: %v", err)
	}

	duplicate := core.NewRecord(collection)
	duplicate.Set("title", "Duplicate")
	duplicate.Set(core.FieldNameI18nGroupId, source.GetString(core.FieldNameI18nGroupId))
	duplicate.Set(core.FieldNameLocale, "vi")
	if err := app.Save(duplicate); err == nil {
		t.Fatalf("Expected duplicate locale translation to fail")
	}
}

func TestI18nEnableCollectionBackfillsExistingRecordsBeforeUniqueIndex(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("i18n_existing_posts")
	collection.Fields.Add(&core.TextField{Name: "title"})
	if err := app.Save(collection); err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	first := core.NewRecord(collection)
	first.Set("title", "First")
	if err := app.Save(first); err != nil {
		t.Fatalf("Failed to create first record: %v", err)
	}

	second := core.NewRecord(collection)
	second.Set("title", "Second")
	if err := app.Save(second); err != nil {
		t.Fatalf("Failed to create second record: %v", err)
	}

	collection, err := app.FindCollectionByNameOrId(collection.Id)
	if err != nil {
		t.Fatalf("Failed to reload collection: %v", err)
	}
	collection.I18n.Enabled = true
	collection.I18n.DefaultLocale = core.DefaultLocaleCode
	collection.I18n.LocalizedFields = []string{"title"}
	if err := app.Save(collection); err != nil {
		t.Fatalf("Failed to enable i18n on existing collection: %v", err)
	}

	records, err := app.FindAllRecords(collection)
	if err != nil {
		t.Fatalf("Failed to load records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	groups := map[string]struct{}{}
	for _, record := range records {
		groupId := record.GetString(core.FieldNameI18nGroupId)
		if groupId == "" {
			t.Fatalf("Expected record %q to have i18n group id", record.Id)
		}
		if record.GetString(core.FieldNameLocale) != core.DefaultLocaleCode {
			t.Fatalf("Expected default locale on record %q, got %q", record.Id, record.GetString(core.FieldNameLocale))
		}
		if !record.GetBool(core.FieldNameIsSource) {
			t.Fatalf("Expected record %q to be marked as source", record.Id)
		}
		groups[groupId] = struct{}{}
	}
	if len(groups) != 2 {
		t.Fatalf("Expected each existing record to have a unique i18n group, got %d groups", len(groups))
	}

	collection, err = app.FindCollectionByNameOrId(collection.Id)
	if err != nil {
		t.Fatalf("Failed to reload localized collection: %v", err)
	}
	if collection.GetIndex("idx_i18n_existing_posts_i18n_unique") == "" {
		t.Fatalf("Expected i18n unique index after backfill")
	}
}
