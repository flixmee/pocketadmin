Goal (incl. success criteria):

- Update the built-in Task Management collection preset to follow the user-supplied exported schema.
- Success means the preset accurately represents the supplied schema, every collection rule requires `@request.auth.id != ""`, focused JSON/preset tests pass, and the completed change is committed and pushed to the current branch.

Constraints/Assumptions:

- Follow `AGENTS.md`; preserve unrelated user changes and keep this ledger current.
- Reuse the existing embedded collection-preset mechanism; no UI change is expected because the picker reads the API catalog dynamically.
- Preset imports remain create-only, use deterministic collection IDs, and do not include sample data in Phase 1.
- Treat the supplied attachment as the source schema and adapt it to the repository's preset format rather than copying runtime-only/system export fields blindly.
- Set `listRule`, `viewRule`, `createRule`, `updateRule`, and `deleteRule` to `@request.auth.id != ""` for every collection; auth-only rules will be assessed against the same request.
- Standard verification is `go test ./...`; focused preset tests are required for this change.

Key decisions:

- Use preset ID `task-management`, display name `Task Management`, and collection group `Task Management`.
- Preserve shared `media` fields as the preset-format equivalent of the supplied legacy `file` fields.
- Add board-scoped `custom_fields` and typed `task_custom_field_values`, including unique board/key and task/field indexes.
- Apply the requested auth expression to the five collection CRUD/API rules; do not apply it to the members `authRule`, because that would prevent signed-out users from logging in.
- Bump the Task Management preset version to `1.1.0`.

State:
  - Done:
    - Updated `task-management.json` from 9 to 11 collections and from 20 to 23 symbolic internal relationships.
    - Confirmed collection names and all non-system field name/type pairs match the supplied schema, with intentional `file` to `media` adaptation.
    - Set `listRule`, `viewRule`, `createRule`, `updateRule`, and `deleteRule` on all 11 collections to `@request.auth.id != ""`.
    - Added the custom-field unique indexes and verified a prefixed real import normalizes and creates them.
    - Updated core/API catalog, preview, relation, rule, index, and import assertions plus feature-plan documentation.
    - JSON parsing, Go formatting, schema comparison, and `git diff --check` passed.
    - `GOCACHE=/tmp/pocketadmin-go-cache go test ./core/presets -count=1` passed.
    - `GOCACHE=/tmp/pocketadmin-go-cache go test ./apis -run '^TestCollectionPreset' -count=1` passed.
    - Full `go test ./...` was run outside the sandbox and still failed in unrelated existing tests: API SQL/OTP behavior and migratecmd generated snapshot IDs.
  - Now:
    - Completed and verified Task Management preset is ready for delivery on branch `presets`.
  - Next:
    - Restart/rebuild any running server binary so the updated embedded preset is available.

Open questions (UNCONFIRMED if needed):

- None currently blocking.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `core/presets/task-management.json`
- `core/presets/preset_test.go`
- `apis/collection_preset_test.go`
- `PocketBase_Collection_Presets_Feature_Plan.md`
- `/Volumes/MacOS_WD/Users/hungtrancongvinh/hungtrancongvinh/.codex/attachments/1559c3eb-eaba-4405-a9c3-8e772f47c8b2/pasted-text.txt`
- `GOCACHE=/tmp/pocketadmin-go-cache go test ./core/presets -count=1`
- `GOCACHE=/tmp/pocketadmin-go-cache go test ./apis -run '^TestCollectionPreset' -count=1`
- `GOCACHE=/tmp/pocketadmin-go-cache go test ./...`
