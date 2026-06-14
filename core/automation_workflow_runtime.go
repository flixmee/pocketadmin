package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

type AutomationResumeInput struct {
	Token string         `json:"token,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type AutomationApprovalDecision struct {
	Decision string         `json:"decision"`
	Comment  string         `json:"comment,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
}

type automationResumeOptions struct {
	BranchKey string
}

func createAutomationWorkflowState(app App, automation *Automation, run *AutomationRun, payload automationTriggerPayload) (*WorkflowState, error) {
	state := NewWorkflowState(app)
	state.SetAutomationRef(automation.Id)
	state.SetRunRef(run.Id)
	state.SetStatus(WorkflowStateStatusRunning)
	state.SetCurrentStepIndex(-1)

	contextRaw, err := toJSONRaw(payload)
	if err != nil {
		return nil, err
	}
	state.SetContext(contextRaw)

	emptyRaw, err := toJSONRaw([]AutomationStepResult{})
	if err != nil {
		return nil, err
	}
	state.SetCheckpoints(emptyRaw)

	if raw, err := toJSONRaw(map[string]any{}); err == nil {
		state.SetWaitingFor(raw)
	}

	if err := app.Save(state); err != nil {
		return nil, err
	}

	return state, nil
}

func persistAutomationWorkflowCheckpoint(app App, state *WorkflowState, run *AutomationRun, stepResults []AutomationStepResult) error {
	if state == nil || run == nil {
		return nil
	}

	if len(stepResults) > 0 {
		state.SetCurrentStepIndex(stepResults[len(stepResults)-1].Index)
		raw, err := toJSONRaw(stepResults)
		if err != nil {
			return err
		}
		state.SetCheckpoints(raw)
		run.SetStepResults(raw)
	}

	if err := app.Save(state); err != nil {
		return err
	}

	return app.Save(run)
}

func findAutomationWorkflowStateByRun(app App, runId string) (*WorkflowState, error) {
	return app.FindWorkflowStateByRunRef(runId)
}

// ResumeAutomationWorkflowState resumes a waiting workflow from its stored state.
func (app *BaseApp) ResumeAutomationWorkflowState(stateID string, input AutomationResumeInput) error {
	state, err := app.FindWorkflowStateById(stateID)
	if err != nil {
		return err
	}

	if token := strings.TrimSpace(input.Token); token != "" && token != state.ResumeToken() {
		return errors.New("invalid workflow resume token")
	}

	return app.resumeAutomationWorkflowState(state, input.Input)
}

// ResumeAutomationWorkflowByToken resumes a waiting workflow using its public resume token.
func (app *BaseApp) ResumeAutomationWorkflowByToken(token string, input map[string]any) error {
	state, err := app.FindWorkflowStateByResumeToken(token)
	if err != nil {
		return err
	}

	return app.resumeAutomationWorkflowState(state, input)
}

func (app *BaseApp) resumeAutomationWorkflowState(state *WorkflowState, input map[string]any) error {
	return app.resumeAutomationWorkflowStateWithOptions(state, input, automationResumeOptions{})
}

func (app *BaseApp) resumeAutomationWorkflowStateWithOptions(state *WorkflowState, input map[string]any, options automationResumeOptions) error {
	if state.Status() != WorkflowStateStatusWaiting {
		return fmt.Errorf("workflow state %q is not waiting", state.Id)
	}

	run, err := app.FindAutomationRunById(state.RunRef())
	if err != nil {
		return err
	}

	automation, err := app.FindAutomationById(state.AutomationRef())
	if err != nil {
		return err
	}
	if snapshotAutomation, err := applyAutomationVersionSnapshot(automation, run.WorkflowVersionSnapshot()); err == nil {
		automation = snapshotAutomation
	}

	payload, err := decodeAutomationRunPayload(run)
	if err != nil {
		return err
	}
	hydrateAutomationTriggerRecords(app, &payload)
	if input != nil {
		payload.Request = mergeAutomationResumeInput(payload.Request, input)
	}

	run.SetStatus(AutomationRunStatusRunning)
	run.SetError("")
	run.ClearErrorStepIndex()
	state.SetStatus(WorkflowStateStatusRunning)
	state.SetResumeToken("")
	if raw, err := toJSONRaw(map[string]any{}); err == nil {
		state.SetWaitingFor(raw)
	}

	if err := app.Save(run); err != nil {
		return err
	}
	if err := app.Save(state); err != nil {
		return err
	}

	ctx := newAutomationExecutionContext(app, automation, run, payload)
	ctx.State = state
	ctx.StartStepIndex = state.CurrentStepIndex() + 1
	if options.BranchKey != "" {
		steps, err := decodeAutomationSteps(automation.Steps().String())
		if err != nil {
			return err
		}
		index := state.CurrentStepIndex()
		if index >= 0 && index < len(steps) {
			ctx.ResumeBranches = automationStepBranchSteps(steps[index], options.BranchKey)
		}
	}
	setCurrentAutomationPolicyContext(app, run)
	defer clearCurrentAutomationPolicyContext(app, run.Id)

	stepResults, execErr := executeAutomationSteps(ctx)
	if isAutomationPauseError(execErr) {
		return persistAutomationWorkflowCheckpoint(app, state, run, stepResults)
	}

	return finalizeAutomationRun(app, automation, run, stepResults, execErr)
}

// ResolveAutomationApproval records an approval decision and resumes or fails the workflow.
func (app *BaseApp) ResolveAutomationApproval(approvalID string, decision AutomationApprovalDecision) error {
	approval, err := app.ResolveAutomationApprovalDecision(approvalID, decision)
	if err != nil {
		return err
	}

	return app.continueAutomationApprovalWorkflow(approval, decision.Input)
}

// ResolveAutomationApprovalDecision records an approval decision without continuing the workflow.
func (app *BaseApp) ResolveAutomationApprovalDecision(approvalID string, decision AutomationApprovalDecision) (*Approval, error) {
	approval, err := app.FindApprovalById(approvalID)
	if err != nil {
		return nil, err
	}
	if approval.Status() != ApprovalStatusPending {
		return nil, errors.New("approval has already been resolved")
	}

	normalized := strings.ToLower(strings.TrimSpace(decision.Decision))
	switch normalized {
	case ApprovalStatusApproved, "approve":
		normalized = ApprovalStatusApproved
	case ApprovalStatusRejected, "reject":
		normalized = ApprovalStatusRejected
	default:
		return nil, errors.New("approval decision must be approved or rejected")
	}

	approval.SetStatus(normalized)
	approval.SetDecision(normalized)
	approval.SetComment(decision.Comment)
	approval.SetResolved(types.NowDateTime())
	if err := app.Save(approval); err != nil {
		return nil, err
	}
	if _, err := app.ResolveApprovalNotifications(approval.Id, normalized); err != nil {
		return nil, err
	}

	return approval, nil
}

// ContinueAutomationApproval continues the workflow for an already resolved approval.
func (app *BaseApp) ContinueAutomationApproval(approvalID string, input map[string]any) error {
	approval, err := app.FindApprovalById(approvalID)
	if err != nil {
		return err
	}

	return app.continueAutomationApprovalWorkflow(approval, input)
}

func (app *BaseApp) continueAutomationApprovalWorkflow(approval *Approval, input map[string]any) error {
	switch approval.Status() {
	case ApprovalStatusApproved:
		return app.resumeAutomationApprovalWorkflow(approval, input, "true")
	case ApprovalStatusRejected:
		hasFalseBranch, err := app.approvalWorkflowHasBranch(approval, "false")
		if err != nil {
			return err
		}
		if hasFalseBranch {
			return app.resumeAutomationApprovalWorkflow(approval, input, "false")
		}

		return app.rejectAutomationApprovalWorkflow(approval)
	default:
		return fmt.Errorf("approval %q is not resolved", approval.Id)
	}
}

func (app *BaseApp) resumeAutomationApprovalWorkflow(approval *Approval, input map[string]any, branchKey string) error {
	state, err := app.FindWorkflowStateById(approval.WorkflowStateRef())
	if err != nil {
		return err
	}
	input = mergeAutomationApprovalResumeInput(input, approval, branchKey)
	return app.resumeAutomationWorkflowStateWithOptions(state, input, automationResumeOptions{BranchKey: branchKey})
}

func (app *BaseApp) approvalWorkflowHasBranch(approval *Approval, branchKey string) (bool, error) {
	run, err := app.FindAutomationRunById(approval.RunRef())
	if err != nil {
		return false, err
	}
	automation, err := app.FindAutomationById(approval.AutomationRef())
	if err != nil {
		return false, err
	}
	if snapshotAutomation, err := applyAutomationVersionSnapshot(automation, run.WorkflowVersionSnapshot()); err == nil {
		automation = snapshotAutomation
	}
	steps, err := decodeAutomationSteps(automation.Steps().String())
	if err != nil {
		return false, err
	}
	index := approval.StepIndex()
	if index < 0 || index >= len(steps) {
		return false, nil
	}

	return len(automationStepBranchSteps(steps[index], branchKey)) > 0, nil
}

func mergeAutomationApprovalResumeInput(input map[string]any, approval *Approval, branchKey string) map[string]any {
	if input == nil {
		input = map[string]any{}
	}
	input["approval"] = map[string]any{
		"id":       approval.Id,
		"decision": approval.Decision(),
		"status":   approval.Status(),
		"comment":  approval.Comment(),
		"branch":   branchKey,
	}
	return input
}

func (app *BaseApp) rejectAutomationApprovalWorkflow(approval *Approval) error {
	state, err := app.FindWorkflowStateById(approval.WorkflowStateRef())
	if err != nil {
		return err
	}
	run, err := app.FindAutomationRunById(approval.RunRef())
	if err != nil {
		return err
	}
	automation, err := app.FindAutomationById(approval.AutomationRef())
	if err != nil {
		return err
	}

	state.SetStatus(WorkflowStateStatusFailed)
	run.SetStatus(AutomationRunStatusFailed)
	run.SetError("approval rejected")
	run.SetErrorStepIndex(approval.StepIndex())
	run.SetRaw("finished", types.NowDateTime())

	if err := app.Save(state); err != nil {
		return err
	}
	if err := app.Save(run); err != nil {
		return err
	}

	if err := updateAutomationLastRunState(app, automation.Id, run.Status(), run.Finished()); err != nil {
		return err
	}
	if err := notifyAutomationRunCompletion(app, automation, run); err != nil {
		app.Logger().Warn(
			"Failed to create automation run notification",
			"automationId", automation.Id,
			"runId", run.Id,
			"error", err,
		)
	}

	return nil
}

// FindApprovalById returns a single Approval model by id.
func (app *BaseApp) FindApprovalById(id string) (*Approval, error) {
	result := &Approval{}
	err := app.RecordQuery(CollectionNameApprovals).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ResumeExpiredAutomationWorkflowStates resumes delay waits whose expiration has passed.
func (app *BaseApp) ResumeExpiredAutomationWorkflowStates() error {
	states, err := app.FindAllExpiredWaitingWorkflowStates(types.NowDateTime())
	if err != nil {
		return err
	}

	var combined error
	for _, state := range states {
		if err := app.resumeAutomationWorkflowState(state, nil); err != nil && !errors.Is(err, sql.ErrNoRows) {
			combined = errors.Join(combined, err)
		}
	}

	return combined
}

func mergeAutomationResumeInput(existing map[string]any, input map[string]any) map[string]any {
	if existing == nil {
		existing = map[string]any{}
	}
	existing["resume"] = input
	return existing
}

func automationRunStepResults(run *AutomationRun) []AutomationStepResult {
	if run == nil || strings.TrimSpace(run.StepResults().String()) == "" {
		return nil
	}

	result := []AutomationStepResult{}
	if err := json.Unmarshal([]byte(run.StepResults().String()), &result); err != nil {
		return nil
	}

	return result
}

func isAutomationWaitStep(stepType string) bool {
	switch stepType {
	case AutomationStepWaitDelay, AutomationStepWaitWebhook, AutomationStepWaitEvent, AutomationStepWaitApproval:
		return true
	default:
		return false
	}
}

func isAutomationPauseError(err error) bool {
	if err == nil {
		return false
	}

	var pauseErr *automationPauseError
	return errors.As(err, &pauseErr)
}
