Goal (incl. success criteria):

- Fix notification dropdown row layout when a notification includes approve/reject buttons.
- Success: notification rows with embedded action buttons render cleanly without broken spacing, nested interactive issues, or overlap with mark-read controls.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; implementation is now requested.
- Existing unrelated changes may be present; do not revert them.
- Notification bell belongs in the admin UI top area and should subscribe to realtime changes.
- `_notifications` is a system collection with recipient-scoped records.
- Treat v1 UI as superuser/admin header notifications while keeping backend schema compatible with auth-record recipients.

Key decisions:

- Use existing collection realtime instead of adding custom websocket/SSE plumbing.
- Prefer dedicated `/api/notifications` endpoints for current-recipient list/count/mark-read to avoid overly broad record update permissions.

State:

- Done:
  - Read visible user request and project instructions.
  - Refreshed this ledger for the notification planning task.
  - Inspected `UI_DOCS.md`, `ui/src/base/appHeader.js`, `ui/src/store.js`, `ui/src/pb.js`, `apis/realtime.go`, and existing system collection migrations.
  - Created `NOTIFICATION_PLAN.md` with data model, permissions, backend, realtime, admin UI, tests, rollout, acceptance criteria, and open questions.
  - Implemented `_notifications` system collection migration, core notification model/query/validation helpers, notification API endpoints, admin UI bell/dropdown, realtime subscription, unread badge state, and mark-read actions.
  - Added focused backend/API tests and updated system collection count expectations.
  - Verified focused notification tests and admin UI build.
  - Updated `NOTIFICATION_PLAN.md` with concrete instructions for creating notifications from trusted server-side code, targeting recipients, choosing fields, and using the read/unread endpoints.
  - Added automation `notifyOnCompletion` system field, migration, model helpers, API allow-list support, and admin UI switches.
  - Added automation completion notification creation for success/failure runs, policy failures, and rejected approval failures.
  - Added focused tests for the automation field and success/failure notification behavior.
  - Verified focused Go tests and admin UI build.
  - Updated automation notification `ActionURL` generation to prefer related record routes for record-triggered runs.
  - Added focused regression coverage for record-triggered automation notification links.
  - Documented the record-triggered automation `ActionURL` convention in `NOTIFICATION_PLAN.md`.
  - Verified focused Go test for completion notifications and record action URLs.
  - Added automation approval request notifications for newly-created `wait.approval` approvals.
  - Added approval notification payload fields/actions (`approvalId`, `approvalStatus`, approve/reject action list, etc.) sourced from `_approvals`.
  - Added admin notification dropdown approve/reject buttons that call the existing approval decision endpoint and mark the notification read after success.
  - Documented automation approval notification behavior in `NOTIFICATION_PLAN.md`.
  - Added focused backend coverage for approval notification creation and payload.
  - Verified focused approval tests and admin UI build.
  - Fixed resumed `record.beforeUpdate` approval workflows by hydrating the saved trigger payload back to a live trigger record.
  - Exposed hydrated trigger records to code steps as root `$record` and `trigger.$record`.
  - Added autosave for changed trigger records in resumed before-record code steps, with record automation triggers suppressed for that internal save to avoid recursive approval loops.
  - Added regression coverage for `record.beforeUpdate` + `wait.approval` + code step mutation.
  - Verified focused before-update/approval tests.
  - Fixed notification rows with approve/reject buttons by separating clickable row bodies from non-clickable action rows and using block content wrappers.
  - Verified admin UI build after the notification row layout fix.
- Now:
  - Reporting the notification row UI fix.
- Next:
  - Optionally verify visually in a running admin UI with seeded approval notifications.

Open questions (UNCONFIRMED if needed):

- Whether non-admin auth-record recipients need a first-party UI is UNCONFIRMED; schema will support them.
- Whether notification payloads are user-facing app data, admin dashboard data, or both is UNCONFIRMED.
- Desired retention/cleanup policy is UNCONFIRMED.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `NOTIFICATION_PLAN.md`
- `ui/src/settings/automations/pageAutomationUpsert.js`
- `ui/src/settings/automations/automationUpsertModal.js`
- `core/automation_model.go`
- `core/automation_notification.go`
- `core/automation_runner.go`
- `core/automation_workflow_runtime.go`
- `core/automation_wait.go`
- `core/automation_steps.go`
- `core/automation_events.go`
- `core/automation_templates.go`
- `core/automation_model_test.go`
- `core/automation_runner_test.go`
- `apis/automation.go`
- `migrations/1775000000_automations.go`
- `migrations/1783000000_automation_notifications.go`
- `ui/src/settings/application/aiAccordion.js`
- Focused checks passed: `go test ./core -run 'TestAutomation(CollectionsExist|Fields|RunCompletionNotifications)'`, `go test ./apis -run 'TestAutomations(Create|Update|List|View)'`, `cd ui && npm run build`.
- Focused check passed: `go test ./core -run 'TestAutomation(RunCompletionNotifications|RecordRunCompletionNotificationActionURL)'`.
- Focused check passed: `go test ./core -run 'TestAutomationWaitApproval(Decision|Notifications)'`.
- Focused check passed: `go test ./core -run 'TestAutomation(BeforeRecordUpdateAfterWaitApprovalCanCustomizeRecord|BeforeRecordUpdateCodeStepCanCustomizeRecord|WaitApprovalDecision|WaitApprovalNotifications)'`.
- Broad `go test ./core` failed in existing-looking `TestNotifyWatcher_CollectionsUpdate` / `TestNotifyWatcher_SettingsUpdate` with automation delay scheduler query interference and scheduler goroutine panic during teardown.
- UI check passed: `cd ui && npm run build` (dprint reported a sandbox cache write warning but Vite build completed).
- UI check passed: `cd ui && npm run build` after notification-row action layout fix.
- `ui/src/base/appHeader.js`
- `ui/src/store.js`
- `ui/src/pb.js`
- `ui/src/notifications/notificationBell.js`
- `ui/src/css/notifications.css`
- `apis/notification.go`
- `apis/notification_test.go`
- `core/notification_model.go`
- `core/notification_query.go`
- `core/notification_validate.go`
- `migrations/1782000000_notifications.go`
- Focused checks passed: `go test ./core -run 'TestNotification|TestNotifications|TestFindAllCollections'`, `go test ./apis -run 'TestNotifications|TestCollectionsList|TestCollectionsImport'`, `cd ui && npm run build`.
- Broad `go test ./...` failed on existing-looking rate-limit/automation scheduler test interference, not focused notification tests.
