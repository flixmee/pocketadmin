package core_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestJSONFieldBaseMethods(t *testing.T) {
	testFieldBaseMethods(t, core.FieldTypeJSON)
}

func TestJSONFieldColumnType(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	f := &core.JSONField{}

	expected := "JSON DEFAULT NULL"

	if v := f.ColumnType(app); v != expected {
		t.Fatalf("Expected\n%q\ngot\n%q", expected, v)
	}
}

func TestJSONFieldPrepareValue(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	f := &core.JSONField{}
	record := core.NewRecord(core.NewBaseCollection("test"))

	scenarios := []struct {
		raw      any
		expected string
	}{
		{"null", `null`},
		{"", `""`},
		{"true", `true`},
		{"false", `false`},
		{"test", `"test"`},
		{"123", `123`},
		{"-456", `-456`},
		{"[1,2,3]", `[1,2,3]`},
		{"[1,2,3", `"[1,2,3"`},
		{`{"a":1,"b":2}`, `{"a":1,"b":2}`},
		{`{"a":1,"b":2`, `"{\"a\":1,\"b\":2"`},
		{[]int{1, 2, 3}, `[1,2,3]`},
		{map[string]int{"a": 1, "b": 2}, `{"a":1,"b":2}`},
		{nil, `null`},
		{false, `false`},
		{true, `true`},
		{-78, `-78`},
		{123.456, `123.456`},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%#v", i, s.raw), func(t *testing.T) {
			v, err := f.PrepareValue(record, s.raw)
			if err != nil {
				t.Fatal(err)
			}

			raw, ok := v.(types.JSONRaw)
			if !ok {
				t.Fatalf("Expected string instance, got %T", v)
			}
			rawStr := raw.String()

			if rawStr != s.expected {
				t.Fatalf("Expected\n%#v\ngot\n%#v", s.expected, rawStr)
			}
		})
	}
}

func TestJSONFieldValidateValue(t *testing.T) {
	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")

	scenarios := []struct {
		name        string
		field       *core.JSONField
		record      func() *core.Record
		expectError bool
	}{
		{
			"invalid raw value",
			&core.JSONField{Name: "test"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", 123)
				return record
			},
			true,
		},
		{
			"zero field value (not required)",
			&core.JSONField{Name: "test"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw{})
				return record
			},
			false,
		},
		{
			"zero field value (required)",
			&core.JSONField{Name: "test", Required: true},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw{})
				return record
			},
			true,
		},
		{
			"non-zero field value (required)",
			&core.JSONField{Name: "test", Required: true},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw("[1,2,3]"))
				return record
			},
			false,
		},
		{
			"non-zero field value (required)",
			&core.JSONField{Name: "test", Required: true},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"aaa"`))
				return record
			},
			false,
		},
		{
			"> default MaxSize",
			&core.JSONField{Name: "test"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"`+strings.Repeat("a", (1<<20))+`"`))
				return record
			},
			true,
		},
		{
			"> MaxSize",
			&core.JSONField{Name: "test", MaxSize: 5},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"aaaa"`))
				return record
			},
			true,
		},
		{
			"<= MaxSize",
			&core.JSONField{Name: "test", MaxSize: 5},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"aaa"`))
				return record
			},
			false,
		},
		// --- JSON Schema validation tests ---
		{
			"value matching schema (object with required property)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`{"name":"hello"}`))
				return record
			},
			false,
		},
		{
			"value violating schema (wrong type)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`{"name":123}`))
				return record
			},
			true,
		},
		{
			"value violating schema (missing required)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`{"other":"value"}`))
				return record
			},
			true,
		},
		{
			"null value bypasses schema validation",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`null`))
				return record
			},
			false,
		},
		{
			"empty string value bypasses schema validation",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(``))
				return record
			},
			false,
		},
		{
			"empty object bypasses schema validation",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`{}`))
				return record
			},
			false,
		},
		{
			"schema with number constraints (valid)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"number","minimum":0,"maximum":100}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`50`))
				return record
			},
			false,
		},
		{
			"schema with number constraints (invalid - too high)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"number","minimum":0,"maximum":100}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`150`))
				return record
			},
			true,
		},
		{
			"schema with string pattern (valid)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"string","pattern":"^[a-z]+$"}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"hello"`))
				return record
			},
			false,
		},
		{
			"schema with string pattern (invalid)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"string","pattern":"^[a-z]+$"}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`"HELLO123"`))
				return record
			},
			true,
		},
		{
			"schema with array items type (valid)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"array","items":{"type":"string"}}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`["a","b","c"]`))
				return record
			},
			false,
		},
		{
			"schema with array items type (invalid)",
			&core.JSONField{
				Name:       "test",
				JsonSchema: `{"type":"array","items":{"type":"string"}}`,
			},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`["a",2,"c"]`))
				return record
			},
			true,
		},
		{
			"no schema set (value passes without schema validation)",
			&core.JSONField{Name: "test"},
			func() *core.Record {
				record := core.NewRecord(collection)
				record.SetRaw("test", types.JSONRaw(`{"anything":"goes"}`))
				return record
			},
			false,
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			err := s.field.ValidateValue(context.Background(), app, s.record())

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("Expected hasErr %v, got %v (%v)", s.expectError, hasErr, err)
			}
		})
	}
}

func TestJSONFieldValidateSettings(t *testing.T) {
	testDefaultFieldIdValidation(t, core.FieldTypeJSON)
	testDefaultFieldNameValidation(t, core.FieldTypeJSON)
	testDefaultFieldHelpValidation[core.JSONField](t)

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection := core.NewBaseCollection("test_collection")

	scenarios := []struct {
		name         string
		field        func() *core.JSONField
		expectErrors []string
	}{
		{
			"MaxSize < 0",
			func() *core.JSONField {
				return &core.JSONField{
					Id:      "test",
					Name:    "test",
					MaxSize: -1,
				}
			},
			[]string{"maxSize"},
		},
		{
			"MaxSize = 0",
			func() *core.JSONField {
				return &core.JSONField{
					Id:   "test",
					Name: "test",
				}
			},
			[]string{},
		},
		{
			"MaxSize > 0",
			func() *core.JSONField {
				return &core.JSONField{
					Id:      "test",
					Name:    "test",
					MaxSize: 1,
				}
			},
			[]string{},
		},
		{
			"MaxSize > safe json int",
			func() *core.JSONField {
				return &core.JSONField{
					Id:      "test",
					Name:    "test",
					MaxSize: 1 << 53,
				}
			},
			[]string{"maxSize"},
		},
		// --- JSON Schema settings validation tests ---
		{
			"valid JSON Schema",
			func() *core.JSONField {
				return &core.JSONField{
					Id:         "test",
					Name:       "test",
					JsonSchema: `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
				}
			},
			[]string{},
		},
		{
			"empty JSON Schema (allowed)",
			func() *core.JSONField {
				return &core.JSONField{
					Id:         "test",
					Name:       "test",
					JsonSchema: "",
				}
			},
			[]string{},
		},
		{
			"invalid JSON Schema (not valid JSON)",
			func() *core.JSONField {
				return &core.JSONField{
					Id:         "test",
					Name:       "test",
					JsonSchema: `{"type":`,
				}
			},
			[]string{"jsonSchema"},
		},
		{
			"invalid JSON Schema (invalid type keyword)",
			func() *core.JSONField {
				return &core.JSONField{
					Id:         "test",
					Name:       "test",
					JsonSchema: `{"type":"notavalidtype"}`,
				}
			},
			[]string{"jsonSchema"},
		},
		{
			"JSON Schema exceeding 50KB size limit",
			func() *core.JSONField {
				return &core.JSONField{
					Id:         "test",
					Name:       "test",
					JsonSchema: `{"type":"string","description":"` + strings.Repeat("a", 51*1024) + `"}`,
				}
			},
			[]string{"jsonSchema"},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			errs := s.field().ValidateSettings(context.Background(), app, collection)

			tests.TestValidationErrors(t, errs, s.expectErrors)
		})
	}
}

func TestJSONFieldCalculateMaxBodySize(t *testing.T) {
	testApp, _ := tests.NewTestApp()
	defer testApp.Cleanup()

	scenarios := []struct {
		field    *core.JSONField
		expected int64
	}{
		{&core.JSONField{}, core.DefaultJSONFieldMaxSize},
		{&core.JSONField{MaxSize: 10}, 10},
	}

	for i, s := range scenarios {
		t.Run(fmt.Sprintf("%d_%d", i, s.field.MaxSize), func(t *testing.T) {
			result := s.field.CalculateMaxBodySize()

			if result != s.expected {
				t.Fatalf("Expected %d, got %d", s.expected, result)
			}
		})
	}
}
