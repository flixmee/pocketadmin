Goal (incl. success criteria):

- Fix Monaco editor snippets for send mail, render template, and fetch single record.
- Success: snippets insert literal dollar-prefixed APIs such as `$template`, `$app`, and `${__hooks}` while preserving intended snippet placeholders.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Keep the change focused in `ui/src/base/monacoEditor.js`.

Key decisions:

- Extend the existing Monaco completion provider with snippet support instead of adding another provider.
- Offer snippets only for JavaScript/TypeScript-like Monaco models.

State:
  - Done:
    - Read `CONTINUITY.md`, `UI_DOCS.md`, `ui/src/base/monacoEditor.js`, and related autocomplete call sites.
    - Confirmed `monacoEditor` already owns Monaco completion registration.
    - Added built-in JavaScript/TypeScript Monaco snippets for send mail, render template, and fetch single record.
    - Preserved existing custom autocomplete items and merged snippets into the same provider.
    - Ran `npx dprint fmt src/base/monacoEditor.js` from `ui/`; passed.
    - Ran `npm run build` from `ui/`; passed with Vite's existing large chunk warning.
    - Reverted generated `ui/dist/index.html` asset hash churn from the verification build.
    - User reported literal dollar-prefixed APIs are not displayed when snippets are inserted.
    - Escaped literal dollar signs in snippet insert text for `$app`, `$template`, and `${__hooks}`.
    - Ran `npx dprint fmt src/base/monacoEditor.js` from `ui/`; passed.
    - Ran `npm run build` from `ui/`; passed with Vite's existing large chunk warning.
    - Reverted generated `ui/dist/index.html` asset hash churn from the verification build.
  - Now:
    - Ready to report implementation and verification.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/base/monacoEditor.js`
- `ui/package.json`
- `npx dprint fmt src/base/monacoEditor.js`
- `npm run build`
