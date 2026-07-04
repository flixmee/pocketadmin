Goal (incl. success criteria):

- Update `ui/src/records/recordUpsertModal.js` so the record upsert form does not apply saved rearrange/form layout preferences.
- Success: record fields render in collection field order/default full-width layout; `recordUpsertModal` no longer reads or applies `collection.rearrange`; targeted checks pass.

Constraints/Assumptions:

- Follow `AGENTS.md`.
- Existing unrelated changes are present; do not revert them.
- When updating the UI, refer to `UI_DOCS.md`.
- Never introduce native HTML `<select>` elements in the admin UI.
- Avoid storing ephemeral presentation state on reactive `field`/`collection` objects when local/out-of-band state is enough.

Key decisions:

- Persist section metadata under `collection.rearrange.sections` and keep flattened `collection.rearrange.layout` for backward-compatible consumers.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reset active ledger state for the `recordUpsertModal` no-rearrange request.
    - Inspected `recordUpsertModal.js` and confirmed it currently reads `readLayoutPreference()` and renders through `normalizeSections()`.
    - Updated `recordUpsertModal.js` to import only `formFields`, stop reading form layout preferences, and render regular fields in collection field order as full-width rows.
    - Updated `docs/collection-rearrange-api.md` to clarify `rearrange` metadata is not applied by the record create/update modal.
    - Ran `npm run build` in `ui/`; build passed with the existing dprint cache warning under `~/Library/Caches`.
    - Confirmed `recordUpsertModal.js` has no remaining `readLayoutPreference`/`normalizeLayout`/`normalizeSections`/`rearrange` references.
    - Ran `git diff --check -- ui/src/records/recordUpsertModal.js docs/collection-rearrange-api.md CONTINUITY.md`; passed.
  - Now:
    - Ready to report the no-rearrange upsert modal update.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/records/recordUpsertModal.js`
- `ui/src/records/recordFormLayoutModal.js`
- `docs/collection-rearrange-api.md`
- `npm run build` from `ui/`
- `git diff --check -- ui/src/records/recordUpsertModal.js docs/collection-rearrange-api.md CONTINUITY.md`
