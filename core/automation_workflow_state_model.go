package core

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/pocketbase/dbx"
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

// FindWorkflowStateByRunRef returns a single WorkflowState model by its automation run id.
func (app *BaseApp) FindWorkflowStateByRunRef(runId string) (*WorkflowState, error) {
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

// FindWorkflowStateByResumeToken returns a waiting WorkflowState model by resume token.
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

// FindAllWorkflowStates returns all WorkflowState models ordered by last update.
func (app *BaseApp) FindAllWorkflowStates() ([]*WorkflowState, error) {
	result := []*WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		OrderBy("updated DESC").
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindAllExpiredWaitingWorkflowStates returns waiting WorkflowState models with an expired wait deadline.
func (app *BaseApp) FindAllExpiredWaitingWorkflowStates(now types.DateTime) ([]*WorkflowState, error) {
	result := []*WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{"status": WorkflowStateStatusWaiting}).
		AndWhere(dbx.NewExp("[[expires]] != '' AND [[expires]] <= {:now}", dbx.Params{"now": now.String()})).
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindNextWaitingWorkflowStateExpiry returns the closest pending delay expiry.
func (app *BaseApp) FindNextWaitingWorkflowStateExpiry(now types.DateTime) (types.DateTime, error) {
	result := &WorkflowState{}
	err := app.RecordQuery(CollectionNameWorkflowState).
		AndWhere(dbx.HashExp{"status": WorkflowStateStatusWaiting}).
		AndWhere(dbx.NewExp("[[expires]] != '' AND [[expires]] > {:now}", dbx.Params{"now": now.String()})).
		OrderBy("expires ASC").
		Limit(1).
		One(result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.DateTime{}, nil
		}
		return types.DateTime{}, err
	}

	return result.Expires(), nil
}
