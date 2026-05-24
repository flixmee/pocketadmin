Goal (incl. success criteria):

- Fix admin UI automation form reactivity after adding a step.
- Success: after adding a step, changing the automation trigger or name still updates/reacts correctly in the UI/save state.

Constraints/Assumptions:

- Follow `AGENTS.md`/`UI_DOCS.md` guidance for UI changes.
- Keep UI changes focused to automations unless investigation shows shared reactive helpers are at fault.
- Do not disturb unrelated dirty work.

Key decisions:

- Preserve the existing reactive `data.form` object when automation steps change; update `data.form.steps` in place instead of replacing `data.form`.

State:
  - Done:
    - Read `CONTINUITY.md` at the start of the turn.
    - User reported: after adding a step, reactivity does not work when changing automation trigger or name.
    - Patched both automation upsert surfaces so step editor `onchange` mutates `data.form.steps` directly.
    - Ran `cd ui && npm run build`; passed. dprint still emitted the existing cache write warning outside the workspace before Vite completed successfully.
  - Now:
    - Ready for user review.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- Exact user-visible failure mode is UNCONFIRMED: likely dirty/change detection or bound field UI not updating after step add.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/recordStepForm.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationUpsertModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/pageAutomationUpsert.js`
- Automation UI files under `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations`
- `cd ui && npm run build`
