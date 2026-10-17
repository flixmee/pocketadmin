Goal (incl. success criteria) [2026-10-17, use_tree option]:
- Base collections get option `use_tree` (bool, JSON key literally `use_tree` per user). Enabling requires a number field named `order`; UI asks user to add it first.

Key decisions:
- Backend: collectionBaseOptions.UseTree + validateTree (error key `use_tree`), wired in validateOptions for base only. Existing i18n base validation is NOT wired (pre-existing) — left untouched.
- Only `order` is required (no parent check) — per user request.

State:
  - Done: core/collection_model_base_options.go, core/collection_validate.go, 4 scenarios in collection_validate_test.go (pass). API verified by curl on scratch :8091.
  - Done: UI Tree section in ui/src/collections/collectionI18nOptionsTab.js (switch name=use_tree; confirm "Add field" inserts order number field onlyInt min 0; warning alert if enabled but field missing). dprint + vite build pass.
  - Failing suites match pre-existing list (TestCollectionDBExport/3_auth fails on HEAD too; logger test flaky).
  - Now: UI not browser-verified (no browser tools); scratch pb :8091 + vite :5174 left running for user check (qa@example.com / QaPass12345!).
  - Done: test build per AGENTS.md (ui dist + examples/base/server linux/amd64) incl. use_tree + ecommerce preset.
  - Done: repo doc docs/collection-tree-api.md (use_tree API + UI + preset category fields); also a Claude Doc write-up (user wanted repo file instead).
  - Next: commit on user approval (ecommerce preset + use_tree still uncommitted).

Working set: core/collection_model_base_options.go, core/collection_validate.go, ui/src/collections/collectionI18nOptionsTab.js

---- Previous ledger ----

Goal (incl. success criteria):
- Add How to Build instructions to AGENTS.md for the UI and Linux amd64 binary.
Constraints/Assumptions:
- Documentation-only; preserve unrelated work.
Key decisions:
- Use the existing examples/base/ directory (user referred to example/base).
State:
  - Done: Confirmed AGENTS.md and examples/base/main.go exist.
  - Done: Added How to Build with npm run build followed by the Linux amd64 Go build; git diff --check passed.
  - Now: Complete.
  - Next: None.
Open questions (UNCONFIRMED if needed):
- None.
Working set (files/ids/commands):
- AGENTS.md, CONTINUITY.md

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-17, blog preset]:
- Blog + E-commerce presets: add image, order, parent to category collections. Success: core/presets tests pass.

State:
  - Done: core/presets/blog.json categories += image (media, 1, images), order (int number, min 0), parent (self relation, max 1); version 1.0.0 -> 1.1.0. Tests updated (core/presets/preset_test.go: 4 relationships, field + import asserts; apis/collection_preset_test.go version). core/presets pass; apis TestCollectionPresetsList still fails on pre-existing "Jobs \u0026 Career" escaping (fails on HEAD too).
  - Done: committed 7b9d5c7f + pushed (develop).
  - Done: ecommerce.json product_categories += image + order (parent already existed); version 1.0.0 -> 1.1.0; preset_test.go asserts added; core/presets pass.
  - Done: test build per AGENTS.md: ui npm run build, then examples/base/server (linux/amd64, CGO_ENABLED=0) with both preset changes.
  - Now: user testing build; ecommerce change not committed.

Working set: core/presets/ecommerce.json, core/presets/blog.json, core/presets/preset_test.go, apis/collection_preset_test.go

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-13, automations editor UI polish]:
- Improve Automations editor page UI: consistent, polished styles for inputs/selects/checkboxes/forms in trigger panel + step drawer.
- Success: visual QA in browser (dark+light), vite build passes, no native <select>.

Constraints/Assumptions:
- Follow AGENTS.md; CSS mostly in ui/src/css/automations.css; user's own server runs on :8090 — use scratch :8091 + vite :5174.
- Uncommitted pre-existing changes (dprint formatting in manualInputs.js/stepEditor.js, pageMedia.js, dist/index.html) — leave alone.

Key decisions:
- Fixed app-wide code-editor overlay misalignment caused by bad merge c10d935a in ui/src/css/form.css (base rules targeted stale `.editor-content`).

State:
  - Done: scratch env (pb binary + data in scratchpad, test superuser in scratchpad/creds.txt); form.css code-editor fix verified.
  - Done: info panel scrolls (flex column, fields overflow-y auto); Notify admins is a real `.field` switch; manual input rows restyled (compact inputs, focus borders, sm checkbox, card bg); field focus borders. Verified dark+light in browser; dprint check + vite build (scratch outDir) pass. Scratch servers stopped.
  - Done: "Add action" drawer grouped by category (stepPaletteGroups in stepEditor.js: Logic/Records/Messaging/Integrations/Wait & approval/AI/Output, unmapped -> Other), per-step descriptions, search (Enter adds first match, reset on open), tinted icons per group, ti-send icon (ti-brand-telegram was undefined). Verified dark+light; build ok.
  - Done: flow node "+" source placeholder now opens the Add action drawer (insert position kept) instead of inserting a Condition step; verified in browser.
  - Done (committed abb4fae6, pushed): UI polish + grouped drawer + "+" opens drawer.
  - Done: new step `execution.data` "Execution data" (manual trigger only): `data` object of templated fields; flow continues; outputs merged across steps (later wins); returned as run output (manual run API/modal) and as `output` of the calling `automation.call` step; precedence: execution data > response body > last step output. Backend core/automation_execution_data.go (+ _test.go, 4 tests pass), wired in steps/validate/schema/call. UI: step option (Output group), form reusing objectRowsEditor, new "JSON" row value type (also fixes object values loading as "[object Object]" in call input rows), ti-database-export icon. Verified end-to-end in browser (save round-trip, direct run, via caller).
  - Now: awaiting user review. Not committed; ui/dist not rebuilt.
  - Next: optional — rebuild ui/dist, further polish of step drawer forms if user wants.

Open questions: none.

Working set: ui/src/css/automations.css, ui/src/css/form.css, ui/src/settings/automations/{stepEditor,manualInputs,pageAutomationUpsert}.js

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-13, automations session]:
- Review Automations; improve Manual trigger (typed input fields, run with input data, optional wait-for-result); add a step that calls another (manual-trigger) automation with input data, in two modes: `wait` (block, use child output) or `parallel` (fire-and-forget, returns child runId).
- Success: go tests for core/apis automation pass incl. new tests; UI build passes; editor supports input fields, call step form, run-with-input modal.

Constraints/Assumptions:
- Follow AGENTS.md; no native <select> in UI; another session may edit concurrently — don't touch unrelated files.
- Callable targets = automations with triggerType `manual`, must be active (DECISION, may revisit).

Key decisions:
- New `_automations.manualInputs` JSON field (migration 1785000000) = [{name,label,type(text|number|boolean|json),required,default,description}].
- Payload gets `Input` (template root `input`, also `trigger.input`) and `Caller` (`trigger.caller`).
- New step type `automation.call` {automation, mode: wait|parallel, input{}, continueOnError?}; depth/loop guard via existing policy MaxDepth (ParentRunId/Depth).
- Response step allowed for manual trigger too = return value of a called automation.
- POST /api/automations/{id}/run accepts {input, wait}; returns run summary (wait) or {runId,status:"queued"} (202).

State:
  - Done: backend (core/automation_call.go, runner/steps/validate/schema/versions/template pkg, migration 1785000000, apis /run {input,wait} -> 202/200), 8 new tests in core/automation_call_test.go pass; template root `input` whitelisted (core/automation_templates.go).
  - Done (review fixes): delay scheduler double-close panic + goroutine querying closed DB (core/automation_delay_scheduler.go: sync.Once shutdown, IsBootstrapped guard, recover).
  - Pre-existing failures on HEAD (not ours): TestSQLRun, TestSettings_DBExport, TestSettingsMarshalJSON, TestCollectionDBExport, TestNotifyWatcher_*; flaky TestCollectionPresetsList, TestSettingsRegisterTelegramWebhook.
  - Done (UI): manualInputs.js (inputs editor + Run modal w/ wait toggle), call step form in stepEditor.js, Run button on page + list (manual only), autocomplete `input` group, response step allowed for manual ("Return response").
  - Done (review fix): pageAutomationUpsert hydrated form by re-normalizing editor steps -> wait.delay reset to 1m / capability inputs lost on load+save. Now normalizes from API data; change detection compares API payloads.
  - Verified: browser QA on scratch server (:8091/:5174, stopped) — run modal validation + result, call step form, save round-trip byte-identical; automation tests + vet + vite build green. Scratch worktree removed.
  - Now: done; awaiting user review. Not committed.
  - Next: user decision on commit; optional follow-ups (pre-existing failing tests, dead automationUpsertModal.js, global policy context race).

Open questions: none yet.

Working set: core/automation_runner.go, automation_steps.go, automation_validate.go, automation_schema.go, automation_versions.go, automation_template_package.go, new core/automation_call.go, migrations/1785000000_automation_manual_inputs.go, apis/automation.go, ui/src/settings/automations/*.

---- Previous ledger ----

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

---- Node editor connections (2026-10-13, connect-together branch) ----
- Done: remove connections (hover ×, or select + Delete/Backspace); "Add node" creates an unconnected node at viewport center (non-overlapping); manual connect by dragging output -> node/input.
- Model: unconnected nodes saved in new `_automations.detachedSteps` (migration 1785000001), list of step groups, validated loosely, never executed. Editor workspace helpers in stepEditor.js (createWorkspace/connectWorkspaceSteps/disconnectWorkspaceEdge).
- Library fixes (flowdesigner.vanilla.js): per-edge `deletable:false`; Delete key ignored when canvas container focused; connection drop target never found (pointer capture) -> now elementFromPoint + node-body drop.
- Verified: browser QA on scratch :8091/:5174 (stopped); go automation tests + vite build green. NOT committed. User's :8090 server needs restart for migrations.
