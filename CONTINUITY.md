Goal (incl. success criteria):

- Fix clicking true/false branch steps in the automation FlowDesigner so they open the edit dialog.
- Success: branch step clicks resolve nested steps recursively and open the same edit modal as root steps; UI build passes.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Prefer focused changes in existing flow designer / automation UI files.

Key decisions:

- Current implementation is a custom vanilla FlowDesigner, not React Flow; preserve that architecture and add React Flow-like interaction hooks/states.
- Keep animation work compositor-friendly: SVG path updates for lines, `translate`/opacity/box-shadow for node/placeholder motion, no layout-driven array/object churn in reactive state.

State:
  - Done:
    - Received request and current IDE context for `ui/src/base/flowdesigner.vanilla.js`.
    - Read `UI_DOCS.md` and inspected FlowDesigner, automation graph builder, and automation CSS.
    - Added source handle hover previews, animated plus glyphs, branch-aware handle accents, ghost connection glow, and handle-click events.
    - Added branch-aware edge styling/markers/labels for `true` and `false`.
    - Added drag-over-edge insertion preview with dashed pulsing placeholder, nearby node nudges, and invalid-node/edge dimming.
    - Added empty branch `+ Add Action` virtual nodes wired to the existing action drawer.
    - Wired canvas handle clicks, empty placeholder clicks/connects, and edge-drop insertion into `stepEditor.js`.
    - Updated automation FlowDesigner theme/CSS toward a darker polished canvas.
    - Verified `npm run build` from `ui/`; build passed.
    - Restored generated `ui/dist/index.html` hash churn after build; source-only UI changes remain.
    - Restored automation flow surface/node/minimap/control colors and shadows to prior admin UI variables.
    - Kept branch/insertion accent variables for the new UX states using existing `successColor`, `warningColor`, and `primaryColor`.
    - Verified `npm run build` from `ui/` after color rollback; build passed.
    - Restored generated `ui/dist/index.html` hash churn after the second build.
    - Removed `fd-handle__label` markup and CSS from `ui/src/base/flowdesigner.vanilla.js`.
    - Confirmed no `fd-handle__label` references remain.
    - Verified `npm run build` from `ui/`; build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Added hover-only SVG delete affordance to FlowDesigner edges.
    - Added `allowEdgeDelete`, `onEdgeDelete`, and `fd:edgedelete` support.
    - Edge delete is independent from node/Delete-key `allowDelete`.
    - Verified `npm run build` from `ui/`; build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Identified branch step click bug: `openStepEditModal()` only searched root steps by id, so nested true/false branch steps were selected but no modal opened.
    - Changed `openStepEditModal()` to resolve string ids with recursive `findStepById()`.
    - Verified `npm run build` from `ui/`; build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
  - Now:
    - Reporting branch step click fix.
  - Next:
    - Manual UI check: click an existing step inside true/false branch and confirm edit modal opens.

Open questions (UNCONFIRMED if needed):

- Browser smoke verification is UNCONFIRMED because the Browser plugin's required JavaScript control tool was not exposed in this session.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/base/flowdesigner.vanilla.js`
- `ui/src/settings/automations/stepEditor.js`
- `ui/src/settings/automations/pageAutomationUpsert.js`
- `ui/src/settings/automations/automationUpsertModal.js`
- `ui/package.json`
- Check passed: `npm run build` from `ui/`.
