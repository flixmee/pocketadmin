Goal (incl. success criteria):

- Implement JSON Schema (draft-07) validation for PocketBase JSON fields per `JSON_SCHEMA_PLAN_v2.md`.
- Success: backend validates record JSON values against configured schemas; admin UI supports enabling/disabling schema + visual/raw editor; JSON record inputs render a schema-driven form when the schema shape is supported and fall back to the raw editor otherwise; root `Object` schema editing supports a whole-data `Repeated` toggle; all tests pass; UI builds clean.
- Success: backend validates record JSON values against configured schemas; admin UI supports enabling/disabling schema + visual/raw editor; JSON record inputs render a schema-driven form when the schema shape is supported and fall back to the raw editor otherwise; root `Object` schema editing supports a whole-data `Repeated` toggle; schema-backed JSON inputs avoid record writes on every keystroke by committing text/number edits on blur and using more local UI state; all tests pass; UI builds clean.

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
  - Replaced native `<select>` controls in `ui/src/fields/json/schemaEditorModal.js` with the shared dropdown/select component.
  - Updated `ui/src/fields/json/settings.js` with schema toggle, configure button, info banner.
  - Updated `ui/src/fields/json/settings.js` so Schema validation toggling uses local UI state and hidden elements instead of recreating the settings subtree.
  - Re-applied the JSON schema shadow-state fix so `ui/src/fields/json/settings.js` no longer mutates `data.field.jsonSchema` during toggle/edit interactions, and `ui/src/collections/collectionUpsertModal.js` materializes `jsonSchema` only on payload export.
  - Reworked JSON schema UI state again to use an out-of-band `WeakMap` store in `ui/src/fields/json/schemaState.js`, so toggling/editing schema no longer mutates the reactive field object at all.
  - Updated `ui/src/collections/collectionUpsertModal.js` to use normalized cloned collections with schema state applied for `hasChanges`, confirmation, and save/export.
  - Improved `ui/src/fields/json/schemaEditorModal.js` reactivity by storing object-property rows as dedicated stores, so typing/changing a property no longer clones the full `properties` array and rerenders the whole list.
  - Confirmed the current `ui/src/fields/json/input.js` still always renders the raw code editor and needs a schema-aware runtime input path.
  - Updated `ui/src/fields/json/input.js` so supported JSON schemas render form inputs for object fields with primitive properties, primitive-root arrays, and primitive scalar roots; unsupported schema shapes still fall back to the raw code editor.
  - Corrected the `Repeated` interpretation: it now applies to the whole root `Object` payload, not individual object properties.
  - Updated `ui/src/fields/json/schemaEditorModal.js` so root `Object` schemas expose a `Repeated` checkbox that round-trips to/from `{"type":"array","items":{"type":"object",...}}`.
  - Updated `ui/src/fields/json/input.js` so repeated root objects render as an array-of-object form UI instead of falling back to the raw editor.
  - Refactored `ui/src/fields/json/input.js` so schema-backed JSON inputs keep a local root value, buffer text/number edits locally, and commit them to the record on blur instead of on every keystroke.
  - Kept structural array changes and boolean toggles committing explicitly, while removing most schema-form rerenders caused by reactive record writes during typing.
  - Added `docs/json-field-api.md` documenting JSON field collection payloads, record payloads, normalization rules, schema validation, and error shapes.
  - Added schema input styling in `ui/src/css/recordFields.css`.
  - Updated `AGENTS.md` with durable frontend reactivity guidance from the JSON schema UI work, including avoiding ephemeral state on reactive field objects and avoiding whole-array replacement for row-local edits.
  - Updated `AGENTS.md` to require the shared dropdown/select component instead of native HTML `<select>` in the admin UI.
  - Added CSS for schema editor modal in `ui/src/css/recordFields.css`.
  - `cd ui && npm run build` succeeds after the input reactivity refactor; dprint still reports its sandboxed incremental-cache write as `Operation not permitted`.
- Now:
  - Implementation complete.
- Next:
  - Manual smoke testing of schema-backed JSON typing/blur behavior in the browser.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_test.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_schema.go` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/field_json_schema_test.go` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/fields/json/settings.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/collections/collectionUpsertModal.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/fields/json/schemaState.js` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/fields/json/schemaEditorModal.js` (new)
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/css/recordFields.css`
- `/Volumes/MacOS_WD/Developer/pocketadmin/AGENTS.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/base/select.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/fields/json/input.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/fields/json/schemaEditorModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/docs/json-field-api.md`
- `cd ui && npm run build` (build succeeded; dprint incremental cache write reported `Operation not permitted`)
- `/Users/suytbily/dev/gits/harry/pocketadmin/go.mod`
- `/Users/suytbily/dev/gits/harry/pocketadmin/go.sum`
