Goal (incl. success criteria):

- Update the media picker and media field flow so `Set selection` stores absolute `_medias` file URLs produced by `app.pb.files.getURL(...)`, and remove the client-side `resolveMediaRecordsByPaths` path-resolution flow entirely.

Constraints/Assumptions:

- Keep the change focused to the media field/picker flow and the minimum backend validation needed to accept the new stored values.
- The repo worktree is dirty; avoid reverting unrelated existing work.
- Refer to `UI_DOCS.md` for UI conventions when touching `ui/`.

Key decisions:

- Keep the media picker callback simple by making it emit selected file URLs directly instead of `_medias` records.
- Remove JS path-to-record resolution from the media field input/view and render stored URLs directly.
- Preserve backend compatibility by accepting both legacy media paths and the new absolute `_medias` file URLs during normalization and validation.

State:

- Done:
  - Read `CONTINUITY.md`, `UI_DOCS.md`, `ui/src/media/mediaPickerModal.js`, `ui/src/fields/media/*`, `ui/src/media/utils.js`, `core/field_media.go`, and related tests.
  - Confirmed the prior UI stored logical media paths and relied on `resolveMediaRecordsByPaths`, while the backend validated only normalized `_medias` paths.
  - Updated the media picker to preload and submit normalized file URLs instead of resolved `_medias` records.
  - Removed client-side media path resolution from the media field input/view and switched them to URL-first rendering.
  - Added backend support for normalizing and validating absolute `/api/files/...` media URLs while keeping legacy path support.
  - Ran `npm run build` in `ui/`; Vite build succeeded. `dprint` logged a sandbox cache write warning, but formatting still completed and the build passed.
  - Ran `go test ./core -run 'TestMedia(Field|Path)'`; tests passed.
- Now:
  - Final review and handoff.
- Next:
  - None.

Open questions (UNCONFIRMED if needed):

- Folder selection in the media field picker is now effectively file-only because `app.pb.files.getURL(...)` has no folder equivalent.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/UI_DOCS.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/media/mediaPickerModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/media/utils.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/fields/media/input.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/fields/media/view.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/fields/media/init.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/field_media.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/media_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/field_media_test.go`
- `rg -n "resolveMediaRecordsByPaths|buildMediaPath|NormalizeMediaPath|ResolveMediaRecordByPath" ui/src core`
- `go test ./core -run 'TestMedia(Field|Path)'`
- `npm run build`
