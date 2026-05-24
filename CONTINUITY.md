Goal (incl. success criteria):

- Add `tag` support to automations and update the automation list to support grouping by tag plus filtering by tag and keyword.
- Success: automation edit/create UI can store a tag; automation list can search by keyword, filter by tag, and group entries by tag using existing admin UI patterns.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; keep UI changes focused.
- Existing unrelated backend/UI/dist changes are present; do not revert them.
- Preserve existing automation behavior and payload shape aside from the new tag field.
- Follow admin UI guidance: use shared components, avoid native selects, keep ephemeral view state local.

Key decisions:

- Tag is implemented as a simple optional system text field on `_automations`.
- Automation list filtering/grouping/search is implemented client-side over the admin list payload.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Added backend tag field/API/model/template/version plumbing.
    - Added automation edit/create tag input.
    - Added automation list keyword search, tag filter, and group-by-tag controls.
    - Ran `go test ./core -run 'TestAutomation(CollectionsExist|Fields)|TestAutomationSchemas|TestAutomationTemplate|TestAutomationVersion|TestPublishAutomationVersion|TestWorkflowTemplate'`; passed.
    - Ran `go test ./apis -run 'TestAutomationsList|TestAutomationCreate|TestAutomationViewUpdateDelete'`; passed.
    - Ran `npm run build`; Vite passed, dprint emitted its existing cache write warning outside the workspace.
    - Ran `./node_modules/.bin/vite build`; passed and regenerated `ui/dist`.
  - Now:
    - Ready for user review.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/settings/automations/`
- `/Users/suytbily/dev/gits/harry/pocketadmin/apis/automation.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/automation_model.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/migrations/1781000001_automation_tags.go`
- `go test ./apis ./core` was attempted but this checkout still fails unrelated broader package tests (`TestDefaultRateLimitMiddleware`, `TestRecordAuthWithOTPManualRateLimiterCheck`, `TestNotifyWatcher_SettingsUpdate`, `TestFindCachedCollectionReferences`, `TestFindAllCollections`).
