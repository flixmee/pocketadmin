Goal (incl. success criteria):

- Document the record-table show/hide column feature under `docs/`.
- Success: the guide explains the admin workflow, database persistence, API payload shape, stable field IDs, legacy behavior, and scope limitations accurately.

Constraints/Assumptions:

- Follow `AGENTS.md`; keep this ledger current and preserve unrelated working-tree changes.
- Documentation must match the implemented base-collection `tableFields` behavior.
- `tableFields` is exposed in collection API JSON and stored inside `_collections.options` internally.

Key decisions:

- Add a focused Markdown guide named `docs/show-hide-table-columns.md`.
- Cover both UI users and API integrators.
- Clearly distinguish explicit database configuration (`[]` included) from legacy unconfigured behavior (`null`/missing).

State:
  - Done:
    - Read the prior ledger and inspected existing `docs/` structure and writing conventions.
    - Confirmed current behavior: base collections persist stable field IDs in `tableFields`; configured values are authoritative; the primary key is always displayed.
    - Added `docs/show-hide-table-columns.md` with admin instructions, persistence semantics, API examples, import/export notes, legacy behavior, and limitations.
    - `git diff --check -- CONTINUITY.md docs/show-hide-table-columns.md` passed.
  - Now:
    - Complete; the show/hide columns guide is ready.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `docs/show-hide-table-columns.md`
- `core/collection_model_base_options.go`
- `ui/src/collections/collectionI18nOptionsTab.js`
- `ui/src/records/recordsList.js`
- `git diff --check`
