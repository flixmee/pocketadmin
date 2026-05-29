Goal (incl. success criteria):

- Move the automation Actions drawer to `document.body`.
- Success: Actions drawer/backdrop are body-mounted instead of nested in the FlowDesigner builder, behavior remains the same, and frontend build passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Need inspect existing Automation implementation before deciding exact migration/backward compatibility path.

Key decisions:

- Prefer a real ES module export from `flowdesigner.vanilla.js` and keep `window.FlowDesigner` only as a compatibility/demo global.
- Automation `stepEditor.js` should import `FlowDesigner` directly instead of checking only `window.FlowDesigner`.
- Use option name `defaultZoom` for the constructor-level initial scale.
- Avoid passing unused reactive dialog/drawer state into the FlowDesigner visual builder; keep the builder mounted across modal state changes.
- Space key should behave as a temporary pan modifier and take priority over node dragging while held.
- Do not render synthetic `automation-placeholder` / "Add step" nodes in automation graph output.
- Node selection can still happen on mouse-down, but node click callbacks should be emitted on mouse-up only if pointer movement stays under a small threshold.
- Keep action palette behavior, but present it as local visual-builder drawer state so FlowDesigner canvas is not remounted by drawer open/close.
- Mount the Actions drawer via a lightweight body portal and remove it when the visual builder unmounts.

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
    - Added `defaultZoom` constructor option to FlowDesigner; value is converted to a number and clamped between existing zoom min/max.
    - Verified `npm run build` from `ui/` after adding `defaultZoom`; build succeeded with the same dprint cache warning.
    - Removed unused reactive dialog/drawer/drag/error inputs from the FlowDesigner visual builder invocation so opening/closing the step modal does not recreate the canvas subtree.
    - Verified `npm run build` from `ui/` after the no-reset fix; build succeeded with the same dprint cache warning.
    - Added Space + left-drag viewport panning to FlowDesigner, including panning from node handles and preserving input/contenteditable typing.
    - Verified `npm run build` from `ui/` after Space + drag panning; build succeeded with the same dprint cache warning.
    - Removed synthetic `automation-placeholder` / "Add step" graph nodes, the click-to-add placeholder path, and matching placeholder CSS.
    - Verified `npm run build` from `ui/` after removing placeholder nodes; build succeeded with the same dprint cache warning.
    - Changed FlowDesigner node click behavior so `onNodeClick`/`fd:nodeclick` fire on mouse-up only when movement stays within a 4px threshold; drag callbacks/change events only fire after the threshold is crossed.
    - Verified `npm run build` from `ui/` after fixing node click vs drag conflict; build succeeded with the same dprint cache warning.
    - Replaced the always-visible Actions side panel with a local right-side drawer opened by an "Add node" button overlaying the FlowDesigner surface.
    - Drawer action clicks add the selected step and close the drawer; closed drawer is inert/hidden from assistive tech.
    - Verified `npm run build` from `ui/` after the Actions drawer change; build succeeded with the same dprint cache warning.
    - Moved the Actions drawer/backdrop into a body-mounted portal and ensured it is removed when the visual builder unmounts.
    - Verified `npm run build` from `ui/` after moving the drawer to body; build succeeded with the same dprint cache warning.
  - Now:
    - Reporting the body-mounted drawer change.
  - Next:
    - Manual browser check: Add node opens a body-level right drawer without clipping and closes correctly.

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
