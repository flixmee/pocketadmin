Goal (incl. success criteria):

- Revert the last polished drag/drop refactor while preserving the earlier working drag behavior.
- Success: `stepEditor.js`, automation drag styling, and `ui/dist` are back to the prior working action-drag/reorder implementation.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; keep UI changes focused.
- Existing unrelated backend/UI/dist changes are present; do not revert them.
- Preserve existing automation behavior and payload shape aside from UI-only drag insertion.
- Follow admin UI guidance: use shared components, avoid native selects, keep ephemeral view state local.

Key decisions:

- Reverted the placeholder-only reorder refactor because the user requested it.
- Keep earlier pointer-based palette insertion and existing node reorder behavior.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Prior tag select work is complete and verified with `npm run build`.
    - Reverted the latest placeholder-based drag/drop refactor from `stepEditor.js` and `automations.css`.
    - Rebuilt `ui/dist` back to the previous asset names (`index-BMkp5Z_3.js`, `index-Do0fcOsO.css`).
    - Ran `npm run build`; passed. dprint still reports the known cache permission warning outside the workspace.
  - Now:
    - Ready for user review.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `ui/src/settings/automations/pageAutomationUpsert.js`
- `ui/src/settings/automations/stepEditor.js`
- `ui/src/css/automations.css`
- `ui/dist/index.html`
- `ui/dist/assets/index-BMkp5Z_3.js`
- `ui/dist/assets/index-Do0fcOsO.css`
- `npm run build`
