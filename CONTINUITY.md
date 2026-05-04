Goal (incl. success criteria):

- Implement JSON Schema (draft-07) validation for PocketBase JSON fields per `JSON_SCHEMA_PLAN_v2.md`.
- Success: backend validates record JSON values against configured schemas; admin UI supports enabling/disabling schema + visual/raw editor; all tests pass; UI builds clean.

Constraints/Assumptions:

- Validation lives inside `core/field_json.go` (ValidateValue / ValidateSettings), not external hooks.
- Use `santhosh-tekuri/jsonschema/v6` Go library for schema compilation and validation.
- Schema stored as `jsonSchema` string property on the JSONField struct.
- Compiled schemas cached in a `sync.Map`, invalidated on collection save.
- Max schema size: 50KB. Max nesting depth: 10 levels.
- Null/empty field values bypass schema validation.
- No retroactive validation of existing records.

Key decisions:

- No hook registration needed — schema validation runs in field's `ValidateValue()` method.
- Schema helpers in dedicated `field_json_schema.go` file.
- v6 library's `AddResource` expects unmarshalled Go values, not `io.Reader`.
- UI schema editor is a modal dialog with visual + raw modes.

State:

- Done:
  - Installed `santhosh-tekuri/jsonschema/v6 v6.0.2` dependency.
  - Created `core/field_json_schema.go` with cache, compile, depth check, error extraction helpers.
  - Modified `core/field_json.go`: added `JsonSchema` field, settings validation, value validation.
  - Created `core/field_json_schema_test.go` with comprehensive helper tests.
  - Updated `core/field_json_test.go` with schema validation test cases.
  - Created `ui/src/fields/json/schemaEditorModal.js` with visual + raw modes.
  - Updated `ui/src/fields/json/settings.js` with schema toggle, configure button, info banner.
  - Added CSS for schema editor modal in `ui/src/css/recordFields.css`.
  - All backend tests pass. UI build succeeds with zero errors.
- Now:
  - Implementation complete.
- Next:
  - Manual smoke testing (user responsibility).

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_test.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_schema.go` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_schema_test.go` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/fields/json/settings.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/fields/json/schemaEditorModal.js` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/css/recordFields.css`
- `/Users/suytbily/dev/gits/harry/pocketadmin/go.mod`
- `/Users/suytbily/dev/gits/harry/pocketadmin/go.sum`
