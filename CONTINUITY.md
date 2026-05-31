Goal (incl. success criteria):

- Add a "URL" tab to the media upload dialog.
- Success: users can paste image URLs separated by line breaks, queue them, and upload them into the existing `_medias` file upload flow.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- URL uploads will use browser `fetch`; remote hosts must allow browser CORS access.
- Keep media upload callbacks/events and parent folder handling consistent with file uploads.

Key decisions:

- Extend `ui/src/media/mediaUploadModal.js` so URL entries use the same queue and submit path as local files.
- Add lightweight CSS in `ui/src/css/media.css` for the upload tabs and URL textarea.

State:
  - Done:
    - Read `CONTINUITY.md` and `UI_DOCS.md`.
    - Located the upload dialog in `ui/src/media/mediaUploadModal.js`.
    - Reviewed existing media upload styles in `ui/src/css/media.css` and shared tabs styling.
    - Added Files/URL tabs to the upload modal.
    - Added newline-separated URL parsing and queueing.
    - Added browser fetch-to-File resolution for URL queue items before the existing `_medias` upload request.
    - Added image type checks, MIME/name inference, per-item fetch errors, and abort handling for URL fetches.
    - Updated upload queue labels and footer button state so pasted URLs can be uploaded directly.
    - Added URL tab/textarea styling in `ui/src/css/media.css`.
    - Ran `npx dprint fmt src/media/mediaUploadModal.js src/css/media.css` from `ui/`; passed.
    - Ran `npm run build` from `ui/`; passed with Vite's existing large chunk warning.
    - Reverted generated `ui/dist/index.html` asset hash churn from the verification build.
    - Browser plugin smoke check was attempted but blocked because the required Node REPL JavaScript execution tool was unavailable.
  - Now:
    - Ready to report implementation and verification.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/media/mediaUploadModal.js`
- `ui/src/css/media.css`
- `UI_DOCS.md`
- `npx dprint fmt src/media/mediaUploadModal.js src/css/media.css`
- `npm run build`
