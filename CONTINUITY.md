Goal (incl. success criteria):

- Fix Presets select state when editing an existing JSON Schema that has `present_name`.
- Success: saved schemas with `present_name` reopen with the matching Presets option selected, selecting a preset leaves it selected, and UI checks pass.

Constraints/Assumptions:

- Follow `AGENTS.md`.
- Existing unrelated changes may be present; do not revert them.
- When updating the UI, refer to `UI_DOCS.md`.
- Never introduce native HTML `<select>` elements in the admin UI. Use `app.components.select`.
- Avoid storing ephemeral presentation state on reactive `field`/`collection` objects when local/out-of-band state is enough.
- In reactive list editors, avoid cloning/reassigning the whole array on each keystroke for row-local edits.
- JSON schema field UI currently lives in `ui/src/fields/json/settings.js`, `ui/src/fields/json/schemaEditorModal.js`, and `ui/src/fields/json/schemaState.js`; collection save/change detection for schema state is normalized in `ui/src/collections/collectionUpsertModal.js`.

Key decisions:

- Implement presets inside the existing JSON Schema editor modal rather than changing backend field payloads.
- Presets are array-of-object schemas matching repeater use cases from `docs/Repeater-Field-Use-Cases.md`.
- Applying a preset immediately loads formatted schema and parses it back into visual state when supported.
- Keep the preset selector visible even for visually unsupported schemas so users can switch back to a supported preset.
- Use the exact user-requested custom JSON Schema metadata key `present_name` at the root of preset-generated schemas.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Replaced prior task ledger state with the JSON Schema presets goal.
    - Read `docs/Repeater-Field-Use-Cases.md`, `UI_DOCS.md`, and the JSON schema editor/settings/state files.
    - Added JSON Schema repeater presets in `ui/src/fields/json/schemaEditorModal.js` for common use cases from the doc, including questions, addresses, contacts, education/work, order/invoice lines, image gallery, key/value metadata, tags, FAQ, and related cases.
    - Added a shared `app.components.select` preset picker in the visual schema editor.
    - Added preset application logic that updates raw schema and visual editor state.
    - Kept the preset picker available when the current schema cannot be represented visually.
    - Added small CSS for preset option summaries in `ui/src/css/recordFields.css`.
    - Ran `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js src/css/recordFields.css` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
    - Ran `git diff --check -- ui/src/fields/json/schemaEditorModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`; passed.
    - Read current ledger and inspected the JSON Schema editor/CSS for the 60/40 layout update.
    - Refactored the root type control into a helper and placed it beside Presets in a `.json-schema-toolbar` row.
    - Added CSS using `3fr 2fr` columns for a 60/40 Presets/Root type split and mobile stacking under 600px.
    - Ran `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js src/css/recordFields.css` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
    - Ran `git diff --check -- ui/src/fields/json/schemaEditorModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`; passed.
    - Read current ledger, `docs/json-field-api.md`, and checked preset/layout identifiers in `schemaEditorModal.js` and `recordFields.css`.
    - Added an "Admin UI schema editor" section to `docs/json-field-api.md`.
    - Documented Visual/Raw modes, the 60/40 Presets/Root type layout, mobile stacking, preset behavior, supported preset categories, and an API headers preset schema example.
    - Clarified that presets are admin UI helpers only and saved collections still store a plain `jsonSchema` string.
    - Ran `git diff --check -- docs/json-field-api.md CONTINUITY.md`; passed.
    - Read current ledger, preset implementation, schema-driven JSON input parser, and current `docs/json-field-api.md`.
    - Added root-level `present_name` to preset-generated schemas using the preset label as the value.
    - Updated the visual schema editor to parse and preserve `present_name` when rebuilding supported schemas.
    - Updated the schema-driven JSON record input parser to allow root-level `present_name`.
    - Updated `docs/json-field-api.md` to document `present_name` and include it in the preset example.
    - Ran `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js src/fields/json/input.js` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
    - Ran `git diff --check -- docs/json-field-api.md ui/src/fields/json/schemaEditorModal.js ui/src/fields/json/input.js CONTINUITY.md ui/dist/index.html`; passed.
    - Read current ledger and inspected preset select state handling in `schemaEditorModal.js`.
    - Added mapping from root `present_name` back to `selectedPreset`.
    - Stopped clearing `selectedPreset` immediately after choosing a preset.
    - Cleared `selectedPreset` only for empty schemas.
    - Ran `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
  - Now:
    - Ready to run final checks and report the Presets select fix.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- Whether future nested repeater presets should expand the visual editor/input parser beyond root repeated objects is UNCONFIRMED; current presets intentionally stay compatible with the existing schema-driven record input.
- Whether the intended key was `preset_name` instead of exact `present_name` is UNCONFIRMED; implemented exact user-requested key `present_name`.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `docs/json-field-api.md`
- `docs/Repeater-Field-Use-Cases.md`
- `UI_DOCS.md`
- `ui/src/fields/json/settings.js`
- `ui/src/fields/json/input.js`
- `ui/src/fields/json/schemaEditorModal.js`
- `ui/src/fields/json/schemaState.js`
- `ui/src/collections/collectionUpsertModal.js`
- `ui/src/css/recordFields.css`
- `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js src/css/recordFields.css`
- `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js src/fields/json/input.js`
- `./node_modules/.bin/dprint fmt src/fields/json/schemaEditorModal.js`
- `npm run build` from `ui/`
- `git diff --check -- ui/src/fields/json/schemaEditorModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`
- `git diff --check -- docs/json-field-api.md CONTINUITY.md`
- `git diff --check -- docs/json-field-api.md ui/src/fields/json/schemaEditorModal.js ui/src/fields/json/input.js CONTINUITY.md ui/dist/index.html`
