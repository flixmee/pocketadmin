package core_test

import (
	"context"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestSlugFieldBaseMethods(t *testing.T) {
	testFieldBaseMethods(t, core.FieldTypeSlug)
}

func TestSlugFieldColumnType(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	f := &core.SlugField{}

	expected := "TEXT DEFAULT '' NOT NULL"

	if v := f.ColumnType(app); v != expected {
		t.Fatalf("Expected\n%q\ngot\n%q", expected, v)
	}
}

func TestSlugFieldPrepareValue(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	f := &core.SlugField{}
	record := core.NewRecord(core.NewBaseCollection("test"))

	scenarios := []struct {
		raw      any
		expected string
	}{
		{"", ""},
		{"Hello World", "hello-world"},
		{"  Many   Spaces  ", "many-spaces"},
		{"hello_world", "hello-world"},
		{123.456, "123-456"},
	}

	for _, s := range scenarios {
		t.Run(s.expected, func(t *testing.T) {
			v, err := f.PrepareValue(record, s.raw)
			if err != nil {
				t.Fatal(err)
			}

			if vStr, ok := v.(string); !ok {
				t.Fatalf("Expected string instance, got %T", v)
			} else if vStr != s.expected {
				t.Fatalf("Expected %q, got %q", s.expected, vStr)
			}
		})
	}
}

func TestSlugFieldValidateValue(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")

	scenarios := []struct {
		name        string
		field       *core.SlugField
		record      func() *core.Record
		expectError bool
	}{
		{
			"invalid raw value",
			&core.SlugField{Name: "slug"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", 123)
				return record
			},
			true,
		},
		{
			"zero field value (not required)",
			&core.SlugField{Name: "slug"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", "")
				return record
			},
			false,
		},
		{
			"zero field value (required)",
			&core.SlugField{Name: "slug", Required: true},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", "")
				return record
			},
			true,
		},
		{
			"valid normalized value",
			&core.SlugField{Name: "slug"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", "hello-world")
				return record
			},
			false,
		},
		{
			"non-normalized raw value still validates after normalization",
			&core.SlugField{Name: "slug"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", "Hello World")
				return record
			},
			false,
		},
		{
			"invalid normalized value",
			&core.SlugField{Name: "slug"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("slug", "!!!")
				return record
			},
			true,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := s.field.ValidateValue(context.Background(), app, s.record())
			if hasErr := err != nil; hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}
		})
	}
}

func TestSlugFieldValidateSettings(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")
	collection.Fields.Add(&core.TextField{Name: "title"})

	scenarios := []struct {
		name        string
		field       *core.SlugField
		expectError bool
		errorKey    string
	}{
		{
			"missing attached field",
			&core.SlugField{Name: "slug", AttachedField: "missing"},
			true,
			"attachedField",
		},
		{
			"self reference",
			&core.SlugField{Name: "slug", Id: "slug_id", AttachedField: "slug_id"},
			true,
			"attachedField",
		},
		{
			"valid attached field",
			&core.SlugField{Name: "slug", AttachedField: "title_id"},
			false,
			"",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			errs, _ := s.field.ValidateSettings(context.Background(), app, collection).(validation.Errors)

			hasErr := errs[s.errorKey] != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, errs)
			}
		})
	}
}

func TestSlugFieldIntercept(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")
	collection.Fields.Add(&core.TextField{Name: "title", Id: "title_id"})

	slugField := &core.SlugField{Name: "slug", Id: "slug_id", AttachedField: "title_id"}
	collection.Fields.Add(slugField)

	scenarios := []struct {
		name       string
		record     func() *core.Record
		expected   string
		actionName string
	}{
		{
			"set attached source on validate",
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("title", "Hello World")
				return record
			},
			"hello-world",
			core.InterceptorActionValidate,
		},
		{
			"preserve manual slug",
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("title", "Hello World")
				record.SetRaw("slug", "custom-slug")
				return record
			},
			"custom-slug",
			core.InterceptorActionValidate,
		},
		{
			"update synced slug when source changes",
			func() *core.Record {
				record := core.NewRecord(collection)
				record.Id = "test"
				record.SetRaw("title", "Old Title")
				record.SetRaw("slug", "old-title")
				if err := record.PostScan(); err != nil {
					panic(err)
				}
				record.SetRaw("title", "New Title")
				return record
			},
			"new-title",
			core.InterceptorActionValidate,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			record := s.record()
			actionCalls := 0

			err := slugField.Intercept(context.Background(), app, record, s.actionName, func() error {
				actionCalls++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			if actionCalls != 1 {
				t.Fatalf("Expected actionCalls %d, got %d", 1, actionCalls)
			}

			if v := record.GetString("slug"); v != s.expected {
				t.Fatalf("Expected value %q, got %q", s.expected, v)
			}
		})
	}
}
