Goal (incl. success criteria):

- Simplify the Collection Group upsert icon selector trigger and remove shared `btn` styling from the select-icon button.
- Success: the icon picker still opens and shows selected/empty state, but the trigger no longer uses the `btn` class; focused UI syntax/format checks pass or failures are reported.


Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`.
- Keep Go and UI changes isolated except where this feature spans both.
- Current icon metadata and Collection Group upsert structures must be confirmed from local files.

Key decisions:

- Store selected collection-group icons as relative public icon paths like `Linear/Archive/archive.svg`.
- Add optional `icon` metadata to `_collection_groups` and expose it through the admin collection group metadata API.
- Keep collection `collectionGroup` values as plain group names; icons are group metadata, not collection fields.
- Implement the picker as a dedicated admin UI modal that loads `icons/meta-data.json` from public assets and uses the existing `app.components.select` dropdown.
- Current task only changes the icon selector trigger styling/markup; no backend/API changes expected.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reset active ledger state for icon picker / Collection Group upsert.
    - Consulted `UI_DOCS.md`.
    - Inspected `ui/public/icons/meta-data.json`; schema is `{ variants: string[], categories: [{ name, icons: string[] }] }`.
    - Confirmed icon files live under `ui/public/icons/<variant>/<category>/<kebab-icon>.svg`.
    - Inspected `collectionGroupUpsertModal`, `collectionsSidebar`, store loading, and collection group API/core model.
    - Added `_collection_groups.icon` to initial schema, repair migration, and a new migration for existing databases.
    - Added collection group metadata APIs/helpers: list returns `{name, icon}`, POST upserts a group, PATCH can rename and update icon.
    - Added `collectionIconPickerModal` that loads `icons/meta-data.json`, filters by variant/category/search, validates icon filename conversion against the provided SVG structure, and returns a relative icon path.
    - Added icon selection/clear controls to Collection Group upsert and wired create/edit group flows to persist selected icons.
    - Updated collection group store normalization, group dropdown rendering, and sidebar group icon rendering.
    - Updated focused Go tests for group metadata and migration column coverage.
    - `node --check` passed for touched UI JS files.
    - Focused Go tests passed:
      - `go test ./core -run 'TestFindAllCollectionGroups|TestRenameAndDeleteCollectionGroup|TestBaseAppRunSystemMigrations|TestBaseAppResetBootstrapState'`
      - `go test ./apis -run TestCollectionGroups`
    - `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build` passed; Vite emitted large chunk warnings and plugin timing info.
    - Full icon metadata path validation passed for all variants/categories/icons.
    - `git diff --check` passed.
    - `go test ./...` was attempted and failed in unrelated existing areas: `apis TestSQLRun/single_write_query` expected `affectedRows:0` but got `1`; automation scheduler cleanup panicked in auth/workflow tests.
    - New request: simplify Collection Group upsert icon selector trigger; do not use `btn` for the select icon button.
    - Inspected `ui/src/collections/collectionGroupUpsertModal.js` and `ui/src/css/collectionModal.css`.
    - Replaced the select icon trigger class with `collection-group-icon-trigger` and added compact custom styling.
    - Added fixed preview wrapper states for selected and empty icons.
    - Ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npx dprint fmt src/collections/collectionGroupUpsertModal.js src/css/collectionModal.css`; passed.
    - `node --check src/collections/collectionGroupUpsertModal.js` passed.
    - `git diff --check` passed.
  - Now:
    - Ready to report simplified selector trigger.

  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/public/icons/meta-data.json`
- `ui/src/collections/collectionIconPickerModal.js`
- `ui/src/collections/collectionGroupUpsertModal.js`
- `ui/src/collections/collectionsSidebar.js`
- `ui/src/collections/collectionUpsertModal.js`
- `ui/src/store.js`
- `ui/src/utils.js`
- `ui/src/css/collectionModal.css`
- `ui/src/css/layout.css`
- `ui/src/main.js`
- `ui/dist/index.html`
- `core/collection_model.go`
- `core/app.go`
- `apis/collection.go`
- `migrations/*collection_group*`
- `apis/collection_test.go`
- `core/base_test.go`
- `core/collection_query_test.go`
