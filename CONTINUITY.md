Goal (incl. success criteria):

- Fix `Uncaught ReferenceError: data is not defined` in `ui/src/fields/json/settings.js`.
- Success: JSON field settings use the component `props` contract, no stale `data.field` references remain, and targeted UI build/checks pass.

Constraints/Assumptions:

- Follow `AGENTS.md`.
- Existing unrelated changes are present; do not revert them.
- When updating the UI, refer to `UI_DOCS.md`.
- Never introduce native HTML `<select>` elements in the admin UI.
- Avoid storing ephemeral presentation state on reactive `field`/`collection` objects when local/out-of-band state is enough.

Key decisions:

- Use `props.field` for JSON schema state because field settings components receive `props` and other JSON settings already use `props.field`.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Inspected `ui/src/fields/json/settings.js`; found `data.field` used in `settings(props)`.
    - Inspected `ui/src/fields/json/schemaState.js`.
    - Reviewed `UI_DOCS.md` settings component contract: settings receive `props.field`.
    - Replaced all `data.field` references in JSON settings with `props.field`.
    - Confirmed no `data.field` references remain in `ui/src/fields/json`.
    - Ran `npm run build` from `ui/`; passed. It still prints the existing dprint cache permission warning under `~/Library/Caches`.
    - Reverted the generated `ui/dist/index.html` asset-reference churn caused by the build; source fix remains.
    - Ran `git diff --check -- ui/src/fields/json/settings.js CONTINUITY.md ui/dist/index.html`; passed.
  - Now:
    - Ready to report the JSON settings fix.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/fields/json/settings.js`
- `ui/src/fields/json/schemaState.js`
- `UI_DOCS.md`
- `npm run build` from `ui/`
- `git diff --check -- ui/src/fields/json/settings.js CONTINUITY.md ui/dist/index.html`
