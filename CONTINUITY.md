Goal (incl. success criteria):

- Redesign the automation workflow builder UI to match the requested 600px three-zone layout.
- Success: top bar is fixed at 48px and single-line; info panel overlays top-left with collapse behavior; flow canvas fills remaining height with dotted grid, compact controls, bottom-centered horizontal nodes, line endpoints, Tabler outline icons, flat styling, 0.5px borders, CSS variable colors, no gradients, no shadows except subtle panel shadow, sentence case text, and minimum 11px fonts.
- Latest adjustment: make automation name and tag editable inline in `.automation-workflow-title-row`; remove name/tag controls from the info panel.
- Latest request: add default ghost output line/+ placeholders for unconnected FlowDesigner source handles.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Use existing admin UI architecture and components where practical.
- Browser smoke verification is UNCONFIRMED unless local tooling makes it available.

Key decisions:

- Focus changes in existing automation builder files rather than introducing a new framework or page.
- Preserve existing FlowDesigner behavior and adapt its surrounding chrome/content styling to the requested layout.
- Use Tabler outline icon classes (`ti ti-*`) in edited automation builder markup.

State:
  - Done:
    - Received request and current IDE context (`ui/package.json`, `core/settings_model.go`).
    - Read prior ledger and replaced it with the new automation workflow builder redesign goal.
    - Located automation builder code in `ui/src/settings/automations/stepEditor.js` and styles in `ui/src/css/automations.css`.
    - Reworked the automation upsert page into a 600px workflow shell with a 48px compact top bar, overlay info panel, and canvas zone.
    - Updated the visual FlowDesigner builder with an in-canvas step counter, no minimap, bottom-aligned graph fitting, circular endpoint connectors, and Tabler-style visible icons.
    - Added workflow-specific CSS variables and flat UI styling with 0.5px borders, dotted canvas, white nodes, and only the info panel shadow.
    - Verified `npm run build` from `ui/`; Vite build passed.
    - Build emitted a sandbox warning because dprint could not write its incremental cache under `~/Library/Caches`, but the command exited successfully.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Started Vite dev server at `http://127.0.0.1:5173/` with escalated permission after sandboxed port binding failed.
    - Moved editable automation name input and tag select into `.automation-workflow-title-row`.
    - Removed name/tag controls from the info panel.
    - Verified `npm run build` from `ui/` after inline edit change; Vite build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Increased `.automation-workflow-builder` height/min-height from 600px to 720px.
    - Verified `npm run build` from `ui/` after height change; Vite build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Changed `fitAutomationFlowView()` to vertically center the graph in the usable canvas area instead of bottom-aligning it on fit/init.
    - Verified `npm run build` from `ui/` after centering change; Vite build passed.
    - Restored generated `ui/dist/index.html` hash churn after build.
    - Added FlowDesigner source placeholders for unconnected right-side source handles: a light horizontal ghost line and subtle bordered/shadowed plus button.
    - Wired the source placeholder click event in the automation visual builder to add a default connected `condition` node at the selected handle/path.
    - Placeholder line/+ controls now disappear automatically once a real outgoing edge exists for the source handle.
    - Verified `npm run build` from `ui/` after placeholder change; Vite build passed with the known sandbox dprint cache warning.
    - Restored generated `ui/dist/index.html` hash churn after build.
  - Now:
    - Ready to report the FlowDesigner placeholder implementation.
  - Next:
    - Optional browser smoke test once an automation fixture/page path is available.

Open questions (UNCONFIRMED if needed):

- Exact automation id/test fixture for visual browser smoke is UNCONFIRMED.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `UI_DOCS.md`
- `ui/src/settings/automations/stepEditor.js`
- `ui/src/base/flowdesigner.vanilla.js`
- `ui/src/css/automations.css`
- Check passed: `npm run build` from `ui/`.
- Dev server: `http://127.0.0.1:5173/` (session 10443).
