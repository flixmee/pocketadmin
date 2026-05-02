# Media Field Plan

## Goal
Add a dedicated `media` field type that lets records store absolute media paths resolved from the `_medias` library instead of storing raw uploaded filenames directly on the record.

## Current baseline
- The backend already has a `_medias` system collection and supporting model logic in `core/media_model.go`.
- The admin UI already has a dedicated media manager at `#/media` for browsing, uploading, organizing, and deleting media records.
- The existing `file` field stores filenames inside the owning record and handles uploads directly on that record.
- There is no dedicated `media` field type in `core/` or `ui/src/fields/` yet.
- Field types are registered through `Fields[FieldTypeX]` in `core/` and `window.app.fieldTypes` in `ui/src/fields/*/init.js`.

## Assumption
- "Media field" means a collection field that stores absolute paths to media items managed through `_medias` and uses the media manager/library as its picker surface.
- If instead the intent is only to rename or extend the current `file` field, the plan should be adjusted before implementation.

## Proposed design
1. Add a new `MediaField` type in `core/`.
2. Model it as a path-backed field whose persisted values are absolute media paths, while enforcing media-specific constraints and UI behavior.
3. Support single and multiple selection, similar to the current `file` field's `MaxSelect` behavior.
4. Reuse `_medias` as the source of truth for path resolution, file metadata, previews, hierarchy, and storage-backed files.
5. Add a dedicated record editor input that opens a media picker modal instead of uploading files directly into the owner record.

## Implementation tasks
- Add the new field type constant and register it in `Fields`.
- Define the `MediaField` struct with shared field properties plus media-specific options such as `MaxSelect`, `Required`, allowed media kinds, and optional allowed mime types.
- Implement `Type`, `GetId`, `SetId`, `GetName`, `SetName`, `GetSystem`, `SetSystem`, `GetHidden`, `SetHidden`, `ColumnType`, `PrepareValue`, `ValidateValue`, and `ValidateSettings`.
- Decide whether `MediaField` should reuse text/file-field helpers for path storage/normalization or keep a dedicated implementation.
- Define the persisted value format for single and multiple absolute paths and keep it stable for API, imports, and migrations.
- Define how picker selections map from `_medias` records to stored absolute paths, including folder path derivation and file path normalization.
- Enforce that stored paths resolve to valid `_medias` items and reject folder paths when the field is configured for files only.
- Extend record expansion and/or view helpers so media-field values can resolve paths back to previews, names, and metadata efficiently in the admin UI.
- Add a new UI field module under `ui/src/fields/media/` with `init.js`, `input.js`, `view.js`, and `settings.js`.
- Implement a media picker modal or drawer that reuses the existing `#/media` browsing concepts but works inside record editing flows.
- Decide whether the picker should allow inline upload into `_medias`, or require users to upload in the media manager first.
- Update record summary/table styling so `media` fields render consistently in lists and detail views.
- Update any schema serialization, import/export, cloning, and migration code that assumes the current fixed field-type set.

## Validation rules to define
- Only absolute paths that resolve to `_medias` items are valid values.
- Respect single vs multiple selection limits.
- Empty values are allowed only when the field is not required.
- Normalize path separators and define a canonical absolute-path format before persisting.
- If media-kind filtering is supported, prevent selecting folder paths or incompatible file paths.
- If mime-type filtering is supported, validate against the resolved media record metadata rather than only the original upload request.
- Define behavior for broken paths: reject on save, preserve stale paths, or null them during fetch/expand.

## UI behavior to define
- Record input should show selected media as thumbnails/cards, not plain path strings.
- Users should be able to add, remove, reorder, and preview selected media items.
- The picker should support search and folder navigation against `_medias`.
- Multiple selection should align with existing admin selection patterns where practical.
- The field settings panel should make the field's source explicit: this field stores absolute paths from the shared media library, not per-record uploads.

## Tests to add
- Field construction and base-method coverage for the new field type.
- Validation tests for allowed paths, selection limits, required behavior, canonicalization, and kind/mime restrictions.
- Value preparation and driver-value tests for single and multiple absolute paths.
- Record-save tests ensuring invalid or unresolved `_medias` paths are rejected.
- Serialization/import/export tests for the new field options and stored values.
- UI tests or targeted integration coverage for media picker selection, path persistence, removal, ordering, and rendering.
- Compatibility tests covering interaction between stored paths and existing `_medias` rename/delete/update behavior.

## Resolved decisions
- Canonical storage format is a slash-prefixed absolute path such as `/hero.png` or `/docs/hero.png`, with normalized `/` separators and no trailing slash.
- The field references shared `_medias` entries and does not copy files into record-local storage.
- Files are selectable by default; folders are selectable only when the field explicitly enables `AllowFolders`.
- Inline upload is out of scope for v1. Users should upload into the shared media library first, then select from the picker.
- The `media` field remains a dedicated field type and does not replace the existing `file` field.
- Media or folder renames do not trigger automatic path propagation across referencing records in v1. Existing unchanged stale paths remain tolerated so unrelated record edits are not blocked, but repicking is required when the field value itself changes.

## Acceptance criteria
- A `media` field can be created, validated, serialized, and round-tripped through collection definitions.
- Records can store one or more absolute paths to `_medias` items through the new field type.
- The admin UI provides field settings, record input, and record display for media references.
- Tests cover backend validation, persistence behavior, and the primary admin UI flows.
- The plan is ready to break into implementation steps without additional design work.
