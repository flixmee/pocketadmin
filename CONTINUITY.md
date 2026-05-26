Goal (incl. success criteria):

- Fix current `monaco-editor` incompatibility with newer Vite/Rolldown worker resolution.
- Success: UI dependency/config/source changes allow the admin UI build to resolve Monaco workers under the current Vite/Rolldown toolchain.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; keep UI changes focused.
- Existing unrelated backend/UI/dist changes may be present; do not revert them.
- Keep Go and frontend changes isolated unless required.
- Network access is restricted; dependency/audit commands may require approval if package registry access is needed.

Key decisions:

- Prefer the smallest dependency/config/source change that matches existing UI patterns and verifies with the UI build.
- Pin `monaco-editor` exactly to `0.53.0`: it resolves the Vite 8/Rolldown worker import failure, while `0.55.1` currently introduces moderate `dompurify` audit findings.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reproduced `npm run build` failure: Rolldown could not resolve `monaco-editor/esm/vs/editor/editor.worker?worker` from `ui/src/base/monacoEditor.js`.
    - Installed and tested `monaco-editor@0.55.1`; build passed, but `npm audit --omit=dev` reported 2 moderate vulnerabilities via `dompurify`.
    - Switched to exact `monaco-editor@0.53.0`; build passed and `npm audit --omit=dev` reported 0 vulnerabilities.
    - Rebuilt `ui/dist`, producing Monaco worker/editor chunks and updating `ui/dist/index.html`.
  - Now:
    - Ready for user review.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `ui/package.json`
- `ui/package-lock.json`
- `ui/dist/index.html`
- `ui/dist/assets/*` generated Monaco/editor chunks
- `npm run build`
- `npm audit --omit=dev`
