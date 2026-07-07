Goal (incl. success criteria):

- Update `docs/collection-rearrange-api.md` to document the record form layout builder changes.
- Success: doc reflects `rearrange.type`, `normal` and `sections` values, the normal right column, extra normal sections, and current admin UI behavior.

Constraints/Assumptions:

- Follow `AGENTS.md`.
- Existing unrelated changes are present; do not revert them.
- When updating the UI, refer to `UI_DOCS.md`.
- Never introduce native HTML `<select>` elements in the admin UI. Use `app.components.select`.
- Avoid storing ephemeral presentation state on reactive `field`/`collection` objects when local/out-of-band state is enough.
- User intent for "right column" is UNCONFIRMED; assume normal mode should expose a fixed side Right column while additional sections stack in the main/left column.
- Existing `ui/src/records/recordFormLayoutModal.js`, `ui/src/css/recordFields.css`, and `CONTINUITY.md` changes are part of the current task chain; preserve them.
- Existing untracked `docs/media-field-api.md` is unrelated; do not touch it.

Key decisions:

- Extend the existing `rearrange` preference object with a `type` property instead of changing the saved `layout`, `sections`, or `hidden` shapes.
- Treat missing/unknown `type` as `sections` for backward compatibility with the current section builder.
- Keep the builder data model section-based; normal mode includes Main and Right column sections and may preserve additional left-column sections.
- Document backend-facing layout values as `normal` and `sections`; "Sections and tabs" is the UI label for `sections`.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reviewed `UI_DOCS.md` component guidance and confirmed shared select usage.
    - Inspected `ui/src/records/recordFormLayoutModal.js` and `ui/src/css/recordFields.css`.
    - Confirmed existing worktree has unrelated `docs/media-field-api.md`.
    - Added a form layout type selector using `app.components.select`.
    - Persisted `rearrange.type` with `normal` and `sections` values.
    - Added normal-mode fixed Main and Right column boards while keeping sections/tabs behavior for the existing builder.
    - Added CSS for the normal two-column builder layout and mobile stacking.
    - Ran `./node_modules/.bin/dprint fmt src/records/recordFormLayoutModal.js src/css/recordFields.css` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
    - Ran `git diff --check -- ui/src/records/recordFormLayoutModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`; passed.
    - Updated normal mode to allow adding extra sections.
    - Kept Main and Right column structural in normal mode; extra sections stack in the left/main column and remain editable/removable/reorderable.
    - Updated normal-mode normalization so extra sections are preserved instead of merged into Main.
    - Updated normal-mode CSS so Right column stays in the side column while extra sections remain in the left column, with mobile stacking.
    - Ran `./node_modules/.bin/dprint fmt src/records/recordFormLayoutModal.js src/css/recordFields.css` from `ui/`; exited 0 with existing dprint cache permission warning.
    - Ran `npm run build` from `ui/`; passed with existing dprint cache permission warning and Vite chunk-size warnings.
    - Reverted generated `ui/dist/index.html` asset-reference churn from the build.
    - Ran `git diff --check -- ui/src/records/recordFormLayoutModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`; passed.
    - Read `docs/collection-rearrange-api.md`.
    - Updated `docs/collection-rearrange-api.md` to document `rearrange.type`, `sections` and `normal` values, the "Sections and tabs" UI label, normal mode `main`/`right` structural ids, and extra normal sections.
    - Updated doc examples, response/request snippets, and curl sample to include `"type": "sections"`.
    - Clarified doc wording so rearrange remains layout metadata and does not imply the record create/update modal applies it.
    - Ran `git diff --check -- docs/collection-rearrange-api.md CONTINUITY.md`; passed.
  - Now:
    - Ready to report the completed documentation update.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- Whether the saved layout type should be named exactly `normal` / `sections` or another backend-facing enum is UNCONFIRMED; using `normal` and `sections` locally with UI label "Sections and tabs".

Working set (files/ids/commands):

- `CONTINUITY.md`
- `UI_DOCS.md`
- `ui/src/records/recordFormLayoutModal.js`
- `ui/src/css/recordFields.css`
- `docs/collection-rearrange-api.md`
- `git status --short`
- `./node_modules/.bin/dprint fmt src/records/recordFormLayoutModal.js src/css/recordFields.css`
- `npm run build` from `ui/`
- `git diff --check -- ui/src/records/recordFormLayoutModal.js ui/src/css/recordFields.css CONTINUITY.md ui/dist/index.html`
- `git diff --check -- docs/collection-rearrange-api.md CONTINUITY.md`
