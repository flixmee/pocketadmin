Goal (incl. success criteria):

- Fix automation visual builder showing "Flow designer failed to load."
- Success: FlowDesigner loads reliably in the Vite/admin UI, the automation visual builder instantiates it without relying on a missing global, and frontend build passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Need inspect existing Automation implementation before deciding exact migration/backward compatibility path.

Key decisions:

- Prefer a real ES module export from `flowdesigner.vanilla.js` and keep `window.FlowDesigner` only as a compatibility/demo global.
- Automation `stepEditor.js` should import `FlowDesigner` directly instead of checking only `window.FlowDesigner`.

State:
  - Done:
    - Read the existing ledger and updated it for the new request to remake the automation builder using `flowdesigner.vanilla.js`.
    - Inspected `UI_DOCS.md`, the automation upsert page/modal, `stepEditor.js`, automation CSS, and the FlowDesigner API.
    - Added FlowDesigner embed options for disabling double-click node creation and delete-key graph deletion, plus selection event emission.
    - Imported `flowdesigner.vanilla.js` from `ui/src/main.js`.
    - Reworked `stepEditor.js` visual mode to render trigger/step/branch/placeholder nodes through FlowDesigner, with palette adds, node selection/editing, branch placeholders, graph connection-based step movement, and position retention while mounted.
    - Added CSS for the FlowDesigner automation surface and node content.
    - Verified `npm run build` from `ui/`; build succeeded. dprint still printed the sandbox cache write warning but continued.
    - Converted `flowdesigner.vanilla.js` from UMD-only side effect to an ES module default export while still assigning `window.FlowDesigner`.
    - Updated `stepEditor.js` to import and instantiate `FlowDesigner` directly.
    - Verified `npm run build` from `ui/` after the load fix; build succeeded with the same dprint cache warning.
  - Now:
    - Reporting the fix.
  - Next:
    - Manual browser smoke test to confirm the builder no longer shows the load failure.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/base/flowdesigner.vanilla.js`
- `ui/src/main.js`
- `ui/src/settings/automations/stepEditor.js`
- `ui/src/css/automations.css`
- `UI_DOCS.md`
- Check passed: `npm run build` from `ui/` (with dprint sandbox cache warning).
