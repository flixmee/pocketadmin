Goal (incl. success criteria):

- Fix automation Webhook response step editing so existing step data is displayed when the edit modal/form opens.
- Success: editing an existing Webhook response step pre-populates its saved values without changing unrelated automation behavior.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; keep UI changes focused.
- Existing unrelated backend/UI/dist changes are present; do not revert them.
- Preserve existing automation behavior and payload shape aside from the Webhook response edit-state fix.
- Follow admin UI guidance: use shared components, avoid native selects, keep ephemeral view state local.

Key decisions:

- Keep the fix focused in the admin UI automation editor unless the data shape requires a backend change.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Prior automation drag/drop work was complete and verified with `npm run build`; dprint had a known cache permission warning outside the workspace.
    - Identified double-normalization of automation editor steps as the Webhook response prefill bug.
    - Patched `createEditorStep("response")` to preserve `statusCodeText`, `headersText`, and `bodyText` when a step is already in editor shape.
    - Ran `npm run build`; passed. dprint again reported the known cache permission warning outside the workspace, then formatted 1 file and Vite built successfully.
  - Now:
    - Ready for user review.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `ui/src/settings/automations/pageAutomationUpsert.js`
- `ui/src/settings/automations/stepEditor.js`
- `ui/dist/index.html`
- `ui/dist/assets/index-CyeY0bv7.js`
- `npm run build`
