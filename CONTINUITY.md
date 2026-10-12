Goal (incl. success criteria):
- Fix startup panic from duplicate POST /api/automation-webhooks/{id} registration; verify automation routes build and tests pass.
Constraints/Assumptions:
- Preserve unrelated changes in apis/settings_credentials.go and ui/src/media/pageMedia.js.
- Follow AGENTS.md; backend-only fix.
Key decisions:
- Remove duplicate route registration at its source; keep all supported webhook methods.
State:
  - Done:
    - Read prior ledger and inspected route registrations; POST webhook registered twice in apis/automation.go.
    - Removed duplicate POST registration; helper retains GET/POST/PUT/PATCH/DELETE.
    - go test ./apis -run TestAutomation -count=1 passed (4.796s); includes router construction and webhook behavior.
    - git diff --check passed.
    - Previous ledger reports Telegram features verified; media UI work remains unfinished (UNCONFIRMED current status).
  - Now:
    - Fix complete and verified.
  - Next:
    - User can restart the server to load the fix.
Open questions (UNCONFIRMED if needed):
- None for this fix.
Working set (files/ids/commands):
- CONTINUITY.md
- apis/automation.go
- apis/automation_test.go
- go test ./apis -run TestAutomation

---- Media page improvements (2026-10-12, separate session) ----
- Goal: improve admin UI Media page. UI-only (ui/src/media/pageMedia.js, mediaFolderModal.js, ui/src/css/media.css).
- Done: safe pb.filter params; per-instance request keys (fixed stuck loader on route change); paginated "Load more" (100/page) + "X of Y" footer; search name+mime; no-flicker background refresh (debounced); instant click selection; Enter/Delete/Backspace shortcuts; file Download/Copy URL/Rename; upload progress panel; empty-state actions + no-results state with Clear search; localStorage try/catch.
- Verified: vite build ok; browser-tested search, no-results, selection, menu, rename modal, delete confirm (cancelled).
- Done (2): marquee drag-to-select (pointer events on .page-content, data-media-id on items, Cmd/Ctrl/Shift adds to selection, empty-area click clears); browser-tested grid + list.
- Next: user review; not committed.
