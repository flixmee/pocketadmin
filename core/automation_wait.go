package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const automationWaitResumeTokenLength = 40

type automationPauseError struct {
	State *WorkflowState
}

func (e *automationPauseError) Error() string {
	return "automation paused"
}

func executeAutomationWaitStep(ctx *automationExecutionContext, step map[string]any, stepIndex int) (string, any, error) {
	if ctx.State == nil {
		return automationStepStatusFailed, nil, errors.New("workflow state is required for wait steps")
	}

	stepType := strings.TrimSpace(toString(step["type"]))
	token := ctx.State.ResumeToken()
	if token == "" {
		token = security.RandomString(automationWaitResumeTokenLength)
	}

	waitingFor := map[string]any{
		"type":      stepType,
		"stepIndex": stepIndex,
		"token":     token,
	}
	output := map[string]any{
		"waiting": true,
		"type":    stepType,
		"token":   token,
	}

	var expires types.DateTime

	switch stepType {
	case AutomationStepWaitDelay:
		duration, err := automationWaitDuration(step["duration"])
		if err != nil || duration <= 0 {
			return automationStepStatusFailed, nil, fmt.Errorf("wait.delay step requires a positive duration")
		}
		resumeAt := types.NowDateTime().Add(duration)
		expires = resumeAt
		waitingFor["resumeAt"] = resumeAt
		output["resumeAt"] = resumeAt
	case AutomationStepWaitWebhook, AutomationStepWaitEvent:
		key := strings.TrimSpace(toString(step["key"]))
		if key == "" {
			return automationStepStatusFailed, nil, fmt.Errorf("%s step requires a key", stepType)
		}
		waitingFor["key"] = key
		output["key"] = key
	case AutomationStepWaitApproval:
		approval, err := createAutomationApproval(ctx, step, stepIndex)
		if err != nil {
			return automationStepStatusFailed, nil, err
		}
		waitingFor["approvalId"] = approval.Id
		waitingFor["role"] = approval.Role()
		waitingFor["assignee"] = approval.Assignee()
		output["approvalId"] = approval.Id
		output["role"] = approval.Role()
		output["assignee"] = approval.Assignee()
	default:
		return automationStepStatusFailed, nil, fmt.Errorf("unsupported wait step type %q", stepType)
	}

	rawWaitingFor, err := toJSONRaw(waitingFor)
	if err != nil {
		return automationStepStatusFailed, nil, err
	}
	ctx.State.SetStatus(WorkflowStateStatusWaiting)
	ctx.State.SetCurrentStepIndex(stepIndex)
	ctx.State.SetResumeToken(token)
	ctx.State.SetWaitingFor(rawWaitingFor)
	if !expires.IsZero() {
		ctx.State.SetExpires(expires)
	}

	ctx.Run.SetStatus(AutomationRunStatusWaiting)
	ctx.Run.SetError("")
	ctx.Run.ClearErrorStepIndex()

	if err := ctx.App.Save(ctx.State); err != nil {
		return automationStepStatusFailed, nil, err
	}
	if err := ctx.App.Save(ctx.Run); err != nil {
		return automationStepStatusFailed, nil, err
	}

	return "waiting", output, &automationPauseError{State: ctx.State}
}

func createAutomationApproval(ctx *automationExecutionContext, step map[string]any, stepIndex int) (*Approval, error) {
	if existing, err := findPendingAutomationApproval(ctx.App, ctx.State.Id, stepIndex); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	approval := NewApproval(ctx.App)
	approval.SetWorkflowStateRef(ctx.State.Id)
	approval.SetAutomationRef(ctx.Automation.Id)
	approval.SetRunRef(ctx.Run.Id)
	approval.SetStepIndex(stepIndex)
	approval.SetAssignee(strings.TrimSpace(toString(step["assignee"])))
	approval.SetRole(strings.TrimSpace(toString(step["role"])))
	approval.SetStatus(ApprovalStatusPending)

	if err := ctx.App.Save(approval); err != nil {
		return nil, err
	}

	return approval, nil
}

func automationWaitDuration(value any) (time.Duration, error) {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, errors.New("empty duration")
		}
		if duration, err := time.ParseDuration(v); err == nil {
			return duration, nil
		}
		if seconds, err := strconv.ParseFloat(v, 64); err == nil && seconds > 0 {
			return time.Duration(seconds * float64(time.Second)), nil
		}
	case float64:
		if v > 0 {
			return time.Duration(v * float64(time.Second)), nil
		}
	case float32:
		if v > 0 {
			return time.Duration(float64(v) * float64(time.Second)), nil
		}
	case int:
		if v > 0 {
			return time.Duration(v) * time.Second, nil
		}
	case int64:
		if v > 0 {
			return time.Duration(v) * time.Second, nil
		}
	}

	return 0, fmt.Errorf("invalid duration")
}

func findPendingAutomationApproval(app App, workflowStateId string, stepIndex int) (*Approval, error) {
	result := &Approval{}
	err := app.RecordQuery(CollectionNameApprovals).
		AndWhere(dbx.HashExp{
			"workflowStateRef": workflowStateId,
			"stepIndex":        stepIndex,
			"status":           ApprovalStatusPending,
		}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
