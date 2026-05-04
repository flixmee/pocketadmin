Goal (incl. success criteria):

- Add repo docs for the collection-groups API under `docs/`, covering the available endpoints and the current create/list/rename/delete behavior accurately.

Constraints/Assumptions:

- `collectionGroup` remains stored on `core.Collection` in camelCase; `_collection_groups` is a registry of selectable names, not a foreign-key refactor.
- System collections must keep their separate sidebar section and not participate in grouping.
- The repo worktree may already contain unrelated changes; do not revert them.
- Standard verification target is focused Go tests plus `cd ui && npm run build`; the full suite has an unrelated cron failure.

Key decisions:

- Keep `collectionGroup` as a root-level collection property and add `_collection_groups` as a selectable registry table.
- Load selectable groups through a collection meta API endpoint and let the UI add new groups inline before saving the collection.
- Auto-register any non-empty `collectionGroup` on collection create/update so the registry stays in sync without a separate save step.
- Handle group rename/delete through dedicated API/core helpers that update all affected collections and the registry together.
- Use native `<details>` toggling for grouped sidebar sections and persist open/closed state in local storage.
- Use a small reusable collection-group upsert modal for both create and rename flows instead of browser prompts.
- Document collection groups as a standalone Markdown API reference in `docs/`, including the fact that group creation is implicit via collection save rather than a dedicated POST endpoint.

State:

- Done:
  - Added `_collection_groups` to the bootstrap schema and extended the existing collectionGroup migration to create/backfill the registry for existing apps.
  - Added `FindAllCollectionGroups` and `EnsureCollectionGroup` to the core app surface, normalized group names on collection save, and auto-registered non-empty groups after collection persistence.
  - Added `GET /api/collections/meta/groups` for loading selectable collection groups.
  - Added `collectionGroups` store loading/silent reload support in the UI.
  - Replaced the collection upsert free-text group input with a select that lists existing groups and provides an inline “Add new group” action.
  - Added core/API helpers and endpoints for renaming and deleting collection groups, including updating all collections assigned to the group.
  - Added edit/remove buttons to named sidebar groups and enabled actual collapse/expand behavior with persisted open state.
  - Added/updated regression coverage for `_collection_groups` bootstrap + rerun migration behavior, collection save sync, the groups API endpoint, and rename/delete group behavior.
  - Ran `go test ./core ./apis`; both passed.
  - Ran `cd ui && npm run build`; Vite build passed. `dprint fmt` emitted the known sandbox cache write warning but did not block the build.
  - Replaced the remaining browser `window.prompt` collection-group create/edit flows with a dedicated reusable UI modal.
  - Added `docs/collection-groups-api.md` with collection-group endpoint and payload documentation.
- Now:
  - Final review and handoff.
- Next:
  - None.

Open questions (UNCONFIRMED if needed):

- Full `go test ./...` previously had one unrelated failure in `tools/cron` (`TestCronStartStop` expected 2 runs and got 3).

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/collection_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1640988000_init.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1762156800_collection_group.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/store.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/collections/collectionUpsertModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/collections/collectionGroupUpsertModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/collections/collectionsSidebar.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/utils.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/docs/collection-groups-api.md`
