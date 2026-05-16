package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameWorkflowState = "_workflowState"

	WorkflowStateStatusRunning   = "running"
	WorkflowStateStatusWaiting   = "waiting"
	WorkflowStateStatusCompleted = "completed"
	WorkflowStateStatusFailed    = "failed"
	WorkflowStateStatusExpired   = "expired"
)

var (
	_ Model        = (*WorkflowState)(nil)
	_ PreValidator = (*WorkflowState)(nil)
	_ RecordProxy  = (*WorkflowState)(nil)
)

// WorkflowState defines durable automation execution state.
type WorkflowState struct {
	*Record
}

func NewWorkflowState(app App) *WorkflowState {
	m := &WorkflowState{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameWorkflowState)
	if err != nil {
		c = NewBaseCollection("@__invalid_workflow_state__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *WorkflowState) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameWorkflowState {
		return errors.New("missing or invalid WorkflowState ProxyRecord")
	}

	return nil
}

func (m *WorkflowState) ProxyRecord() *Record {
	return m.Record
}

func (m *WorkflowState) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *WorkflowState) AutomationRef() string {
	return m.GetString("automationRef")
}

func (m *WorkflowState) SetAutomationRef(id string) {
	m.Set("automationRef", id)
}

func (m *WorkflowState) RunRef() string {
	return m.GetString("runRef")
}

func (m *WorkflowState) SetRunRef(id string) {
	m.Set("runRef", id)
}

func (m *WorkflowState) Status() string {
	return m.GetString("status")
}

func (m *WorkflowState) SetStatus(status string) {
	m.Set("status", status)
}

func (m *WorkflowState) CurrentStepIndex() int {
	return m.GetInt("currentStepIndex")
}

func (m *WorkflowState) SetCurrentStepIndex(index int) {
	m.Set("currentStepIndex", index)
}

func (m *WorkflowState) Context() types.JSONRaw {
	raw, _ := m.GetRaw("context").(types.JSONRaw)
	return raw
}

func (m *WorkflowState) SetContext(context types.JSONRaw) {
	m.Set("context", context)
}

func (m *WorkflowState) Checkpoints() types.JSONRaw {
	raw, _ := m.GetRaw("checkpoints").(types.JSONRaw)
	return raw
}

func (m *WorkflowState) SetCheckpoints(checkpoints types.JSONRaw) {
	m.Set("checkpoints", checkpoints)
}

func (m *WorkflowState) ResumeToken() string {
	return m.GetString("resumeToken")
}

func (m *WorkflowState) SetResumeToken(token string) {
	m.Set("resumeToken", token)
}

func (m *WorkflowState) WaitingFor() types.JSONRaw {
	raw, _ := m.GetRaw("waitingFor").(types.JSONRaw)
	return raw
}

func (m *WorkflowState) SetWaitingFor(waitingFor types.JSONRaw) {
	m.Set("waitingFor", waitingFor)
}

func (m *WorkflowState) Expires() types.DateTime {
	return m.GetDateTime("expires")
}

func (m *WorkflowState) SetExpires(expires types.DateTime) {
	m.SetRaw("expires", expires)
}

func (m *WorkflowState) Created() types.DateTime {
	return m.GetDateTime("created")
}

func (m *WorkflowState) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
