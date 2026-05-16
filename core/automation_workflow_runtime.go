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
	result := &WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{"runRef": runId}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindWorkflowStateById returns a single WorkflowState model by id.
func (app *BaseApp) FindWorkflowStateById(id string) (*WorkflowState, error) {
	result := &WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindWorkflowStateByResumeToken returns a waiting workflow state by resume token.
func (app *BaseApp) FindWorkflowStateByResumeToken(token string) (*WorkflowState, error) {
	result := &WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{
			"resumeToken": strings.TrimSpace(token),
			"status":      WorkflowStateStatusWaiting,
		}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
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
	approval, err := app.FindApprovalById(approvalID)
	if err != nil {
		return err
	}
	if approval.Status() != ApprovalStatusPending {
		return errors.New("approval has already been resolved")
	}

	normalized := strings.ToLower(strings.TrimSpace(decision.Decision))
	switch normalized {
	case ApprovalStatusApproved, "approve":
		normalized = ApprovalStatusApproved
	case ApprovalStatusRejected, "reject":
		normalized = ApprovalStatusRejected
	default:
		return errors.New("approval decision must be approved or rejected")
	}

	approval.SetStatus(normalized)
	approval.SetDecision(normalized)
	approval.SetComment(decision.Comment)
	approval.SetResolved(types.NowDateTime())
	if err := app.Save(approval); err != nil {
		return err
	}

	if normalized == ApprovalStatusApproved {
		return app.ResumeAutomationWorkflowState(approval.WorkflowStateRef(), AutomationResumeInput{Input: decision.Input})
	}

	return app.rejectAutomationApprovalWorkflow(approval)
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

	return updateAutomationLastRunState(app, automation.Id, run.Status(), run.Finished())
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
	states := []*WorkflowState{}
	now := types.NowDateTime()
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{"status": WorkflowStateStatusWaiting}).
		AndWhere(dbx.NewExp("[[expires]] != '' AND [[expires]] <= {:now}", dbx.Params{"now": now.String()})).
		All(&states)
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
