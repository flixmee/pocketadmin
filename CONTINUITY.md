Goal (incl. success criteria):

- Add a popup near the records "Refresh" button that lets users rearrange fields and build the form layout, using `demo/grids/` as the reference.
- Success: popup opens fullscreen from the relevant toolbar, clones/refers to `grid-layout.js` without importing from `demo/`, supports add/rearrange/resize/delete-via-trash form layout building, applies a `demo.html`-like style, integrates with existing UI state/persistence where appropriate, and focused UI checks pass or failures are reported.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`.
- Keep Go and UI changes isolated except where this feature spans both.
- Determine current record toolbar and form rendering behavior from local files before editing.

Key decisions:

- Store form layout as local UI preference under `pbFormLayout_<collection.id>` instead of changing collection schema/API.
- Clone `demo/grids/grid-layout.js` into UI source and import the cloned module, keeping `demo/` as reference material only.
- Persist omitted form fields as part of the local layout preference so dragging a field to trash removes it from the record form until re-added.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reset active ledger state for form layout popup request.
    - Consulted `UI_DOCS.md`.
    - Found the Refresh button in `ui/src/collections/pageCollections.js`.
    - Found record upsert regular fields rendered in `ui/src/records/recordUpsertModal.js` as `col-12` in collection field order.
    - Added `ui/src/records/recordFormLayoutModal.js`.
    - Added a Form layout button next to Refresh in the collections page header.
    - Updated record upsert modal to apply saved local form layout order/widths.
    - Added CSS for the form layout grid/cards in `ui/src/css/recordFields.css`.
    - `node --check` passed for touched JS files.
    - `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build` passed; Vite emitted existing large chunk warnings.
    - `git diff --check` passed.
    - New request: make the popup fullscreen and match `grid-layout.js` demo behavior: add fields, drag/reorder, resize width, and drag to trash to delete.
    - Re-read current `recordFormLayoutModal`, `grid-layout.js`, and related CSS.
    - Updated the modal to fullscreen and import `demo/grids/grid-layout.js` directly.
    - Added a canvas grid, add-field dropdown/buttons, and a trash grid using shared `GridLayout` group handoff.
    - Updated saved form layout preferences to include omitted field ids.
    - Updated record upsert rendering to respect omitted fields.
    - Re-ran `node --check` for touched JS files; passed.
    - Re-ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build`; passed with existing large chunk warnings.
    - Re-ran `git diff --check`; passed.
    - New correction: do not import from `demo/grids/grid-layout.js`; clone/reference it and apply `demo.html` styling.
    - Cloned `demo/grids/grid-layout.js` to `ui/src/base/gridLayout.js`.
    - Switched `recordFormLayoutModal.js` to import `@/base/gridLayout`.
    - Added demo-like FormBuilder title, field count, panel titles, dark canvas, dotted grid, panel cards, accent labels, and trash styling.
    - Re-ran `node --check` for touched JS files; passed.
    - Re-ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build`; passed with existing large chunk warnings.
    - Re-ran `git diff --check`; passed.
    - New bug report: fields cannot move or resize in the form layout popup.
    - Added explicit `dataset` assignment for layout cards before `GridLayout` scans the board, so grid items are registered and drag/resize handles are attached.
    - Re-ran `node --check` for touched JS files; passed.
    - Re-ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build`; passed with existing large chunk warnings.
    - Re-ran `git diff --check`; passed.
  - Now:
    - Ready to report move/resize fix.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `UI_DOCS.md`
- `demo/grids/grid-layout.js`
- `demo/grids/demo.html`
- `ui/src/base/gridLayout.js`
- `ui/src/records/recordFormLayoutModal.js`
- `ui/src/records/recordUpsertModal.js`
- `ui/src/collections/pageCollections.js`
- `ui/src/css/recordFields.css`
- `ui/src/main.js`
- `ui/dist/index.html`
