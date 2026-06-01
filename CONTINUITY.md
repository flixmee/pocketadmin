Goal (incl. success criteria):

- Update `ui/src/base/flowdesigner.vanilla.js` viewport navigation.
- Success: desktop Space+drag and middle-button drag pan naturally; wheel zoom stays zoom; trackpad two-finger scroll pans; touch uses 1 finger for interaction, 2-finger drag for pan, pinch for zoom; inertia works without leaks.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`.
- Keep changes focused to `flowdesigner.vanilla.js` unless verification reveals another required file.
- Prefer Pointer Events API for viewport gestures and precise `preventDefault()`.

Key decisions:

- Implement a dedicated `PanController` class in `ui/src/base/flowdesigner.vanilla.js`.
- Preserve existing FlowDesigner API and route node/edge/selection behaviors through pointer-driven handlers.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Read relevant `UI_DOCS.md` conventions.
    - Began inspecting existing flow designer pan/zoom/event code.
    - Added a `PanController` in `ui/src/base/flowdesigner.vanilla.js`.
    - Switched viewport navigation from mouse-only root/window events to pointer events.
    - Added Space+drag, middle-button drag, trackpad pan heuristic, two-touch pan, pinch zoom separation, pointer capture, and inertia.
    - Updated inline FlowDesigner CSS with `touch-action: none` and active pan cursor state.
    - Ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npx dprint fmt src/base/flowdesigner.vanilla.js`; passed.
    - Ran `node --check src/base/flowdesigner.vanilla.js`; passed.
    - Ran `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build`; passed with Vite's existing large chunk warning.
    - Reverted generated `ui/dist/index.html` asset hash churn from the verification build.
  - Now:
    - Ready to report implementation and verification.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `UI_DOCS.md`
- `ui/src/base/flowdesigner.vanilla.js`
- `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npx dprint fmt src/base/flowdesigner.vanilla.js`
- `node --check src/base/flowdesigner.vanilla.js`
- `env DPRINT_CACHE_DIR=/private/tmp/dprint-cache npm run build`
