package core_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func TestMediasCollectionSchema(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		t.Fatalf("expected %s collection to exist: %v", core.CollectionNameMedias, err)
	}

	if !collection.System {
		t.Fatalf("expected %s to be a system collection", core.CollectionNameMedias)
	}

	expectedFields := []string{"id", "name", "kind", "parent", "file", "mime", "size", "created", "updated"}
	for _, fieldName := range expectedFields {
		if collection.Fields.GetByName(fieldName) == nil {
			t.Fatalf("missing field %q", fieldName)
		}
	}

	columns, err := app.TableColumns(core.CollectionNameMedias)
	if err != nil {
		t.Fatal(err)
	}

	for _, column := range expectedFields {
		if !slicesContain(columns, column) {
			t.Fatalf("expected column %q in %v", column, columns)
		}
	}

	indexes, err := app.TableIndexes(core.CollectionNameMedias)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := indexes["idx_medias_unique_parent_name"]; !ok {
		t.Fatalf("missing unique parent/name index: %v", indexes)
	}

	if _, ok := indexes["idx_medias_parent_kind"]; !ok {
		t.Fatalf("missing parent/kind index: %v", indexes)
	}
}

func TestMediaRecordLifecycle(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		t.Fatal(err)
	}

	root := core.NewRecord(collection)
	root.Set("name", "Root")
	root.Set("kind", core.MediaKindFolder)

	if err := app.Save(root); err != nil {
		t.Fatalf("failed to create root folder: %v", err)
	}

	nested := core.NewRecord(collection)
	nested.Set("name", "Nested")
	nested.Set("kind", core.MediaKindFolder)
	nested.Set("parent", root.Id)

	if err := app.Save(nested); err != nil {
		t.Fatalf("failed to create nested folder: %v", err)
	}

	nested.Set("name", "Nested Renamed")
	if err := app.Save(nested); err != nil {
		t.Fatalf("failed to rename nested folder: %v", err)
	}

	rootUpload, err := filesystem.NewFileFromBytes([]byte("root media"), "root file.txt")
	if err != nil {
		t.Fatal(err)
	}

	rootFile := core.NewRecord(collection)
	rootFile.Set("kind", core.MediaKindFile)
	rootFile.Set("file", rootUpload)

	if err := app.Save(rootFile); err != nil {
		t.Fatalf("failed to upload root file: %v", err)
	}

	if rootFile.GetString("name") != "root file.txt" {
		t.Fatalf("expected original filename as media name, got %q", rootFile.GetString("name"))
	}

	if rootFile.GetString("mime") == "" {
		t.Fatal("expected file mime to be populated")
	}

	if rootFile.GetInt("size") <= 0 {
		t.Fatalf("expected positive file size, got %d", rootFile.GetInt("size"))
	}

	childUpload, err := filesystem.NewFileFromBytes([]byte("nested media"), "nested.txt")
	if err != nil {
		t.Fatal(err)
	}

	childFile := core.NewRecord(collection)
	childFile.Set("name", "Nested File")
	childFile.Set("kind", core.MediaKindFile)
	childFile.Set("parent", nested.Id)
	childFile.Set("file", childUpload)

	if err := app.Save(childFile); err != nil {
		t.Fatalf("failed to upload nested file: %v", err)
	}

	refreshedRootFile, err := app.FindRecordById(collection, rootFile.Id)
	if err != nil {
		t.Fatal(err)
	}

	if refreshedRootFile.GetString("file") == "" {
		t.Fatal("expected stored file key after upload")
	}

	duplicateFolder := core.NewRecord(collection)
	duplicateFolder.Set("name", "Nested Renamed")
	duplicateFolder.Set("kind", core.MediaKindFolder)
	duplicateFolder.Set("parent", root.Id)
	if err := app.Save(duplicateFolder); err == nil {
		t.Fatal("expected duplicate sibling name to fail")
	}

	fileAsParent := core.NewRecord(collection)
	fileAsParent.Set("name", "Invalid child")
	fileAsParent.Set("kind", core.MediaKindFolder)
	fileAsParent.Set("parent", rootFile.Id)
	if err := app.Save(fileAsParent); err == nil || !strings.Contains(err.Error(), "parent") {
		t.Fatalf("expected file parent validation error, got %v", err)
	}

	root.Set("parent", root.Id)
	if err := app.Save(root); err == nil {
		t.Fatal("expected self-parenting to fail")
	}
	root.Set("parent", "")

	root.Set("parent", nested.Id)
	if err := app.Save(root); err == nil {
		t.Fatal("expected descendant-parent cycle to fail")
	}
	root.Set("parent", "")

	folderWithFileUpload, err := filesystem.NewFileFromBytes([]byte("bad"), "bad.txt")
	if err != nil {
		t.Fatal(err)
	}

	invalidFolder := core.NewRecord(collection)
	invalidFolder.Set("name", "Folder With File")
	invalidFolder.Set("kind", core.MediaKindFolder)
	invalidFolder.Set("file", folderWithFileUpload)
	if err := app.Save(invalidFolder); err == nil {
		t.Fatal("expected folder upload validation to fail")
	}

	missingFile := core.NewRecord(collection)
	missingFile.Set("name", "Missing File")
	missingFile.Set("kind", core.MediaKindFile)
	if err := app.Save(missingFile); err == nil {
		t.Fatal("expected missing file validation to fail")
	}
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
