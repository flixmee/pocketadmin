Goal (incl. success criteria):

- Change admin request approval so the admin does not wait for related automation completion.
- Success: approving a request returns promptly, automation work continues in the background, errors are surfaced/logged appropriately without blocking the approval response, and existing tests/builds pass or failures are reported.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- When updating admin UI, consult `UI_DOCS.md`; do not introduce native HTML `<select>`.
- Use existing admin UI architecture and components where practical.
- Browser smoke verification is UNCONFIRMED unless local tooling makes it available.

Key decisions:

- Trace existing approval and automation execution paths before editing.
- Keep the existing core `ResolveAutomationApproval` method synchronous for current callers/tests.
- Split approval handling into decision persistence and workflow continuation so the API can dispatch continuation in the background.

State:
  - Done:
    - Received request to make admin approval continue immediately while automation runs in the background.
    - Read and reset stale ledger context for the current request.
    - Found admin approval endpoint: `POST /api/automations/approvals/{id}/decision` in `apis/automation.go`.
    - Found synchronous core path: `BaseApp.ResolveAutomationApproval` in `core/automation_workflow_runtime.go`.
    - Added `ResolveAutomationApprovalDecision` to record approval status/comment and notification updates without continuing the workflow.
    - Added `ContinueAutomationApproval` to resume/fail the workflow for an already resolved approval.
    - Updated the API endpoint to record the decision synchronously and call `ContinueAutomationApproval` via `routine.FireAndForget`, logging continuation errors.
    - Ran `gofmt` on edited Go files.
    - Updated `TestAutomationApprovalDecision` to assert synchronous decision persistence and eventual background run continuation instead of synchronous workflow hook counts.
    - Focused checks passed: `go test ./apis -run 'TestAutomationApprovalDecision' -count=1`.
    - Focused checks passed: `go test ./core -run 'TestAutomationWaitApprovalDecision|TestAutomationWaitApprovalRejectedBranch|TestAutomationBeforeRecordUpdateAfterWaitApprovalCanCustomizeRecord|TestAutomationWaitApprovalNotifications' -count=1`.
    - Broad check `go test ./...` was attempted; it failed in unrelated OTP/auth and automation delay scheduler cleanup panics (`close of closed channel`, nil DB after cleanup), not in the approval-focused tests.
    - Removed generated untracked test artifact `core/pb_base_app_test_data_dir/`.
  - Now:
    - Ready to report implementation and verification.
  - Next:
    - Optional follow-up: investigate existing `go test ./...` cleanup races separately.

Open questions (UNCONFIRMED if needed):

- Existing broad-suite cleanup panics are outside this change and remain unresolved.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `apis/automation.go`
- `apis/automation_test.go`
- `core/automation_workflow_runtime.go`
- `core/app.go`
- Focused tests passed as listed above.
- Broad `go test ./...` failed due unrelated cleanup panics.
