package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameAutomationRuns = "_automationRuns"

	AutomationRunStatusQueued  = "queued"
	AutomationRunStatusRunning = "running"
	AutomationRunStatusWaiting = "waiting"
	AutomationRunStatusSuccess = "success"
	AutomationRunStatusFailed  = "failed"

	automationRunErrorStepIndexUnset = -1
)

var (
	_ Model        = (*AutomationRun)(nil)
	_ PreValidator = (*AutomationRun)(nil)
	_ RecordProxy  = (*AutomationRun)(nil)
)

// AutomationRun defines a Record proxy for working with the automation runs collection.
type AutomationRun struct {
	*Record
}

// NewAutomationRun instantiates and returns a new blank *AutomationRun model.
func NewAutomationRun(app App) *AutomationRun {
	m := &AutomationRun{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameAutomationRuns)
	if err != nil {
		// this is just to make tests easier since automationRuns is a system collection and it is expected to be always accessible
		// (note: the loaded record is further checked on AutomationRun.PreValidate())
		c = NewBaseCollection("@__invalid_automation_runs__")
	}

	m.Record = NewRecord(c)

	return m
}

// PreValidate implements the [PreValidator] interface and checks
// whether the proxy is properly loaded.
func (m *AutomationRun) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameAutomationRuns {
		return errors.New("missing or invalid AutomationRun ProxyRecord")
	}

	return nil
}

// ProxyRecord returns the proxied Record model.
func (m *AutomationRun) ProxyRecord() *Record {
	return m.Record
}

// SetProxyRecord loads the specified record model into the current proxy.
func (m *AutomationRun) SetProxyRecord(record *Record) {
	m.Record = record
}

// AutomationRef returns the referenced automation record id.
func (m *AutomationRun) AutomationRef() string {
	return m.GetString("automationRef")
}

// SetAutomationRef updates the referenced automation record id.
func (m *AutomationRun) SetAutomationRef(automationId string) {
	m.Set("automationRef", automationId)
}

// TriggerType returns the trigger type that started the run.
func (m *AutomationRun) TriggerType() string {
	return m.GetString("triggerType")
}

// SetTriggerType updates the trigger type that started the run.
func (m *AutomationRun) SetTriggerType(triggerType string) {
	m.Set("triggerType", triggerType)
}

// Status returns the run status.
func (m *AutomationRun) Status() string {
	return m.GetString("status")
}

// SetStatus updates the run status.
func (m *AutomationRun) SetStatus(status string) {
	m.Set("status", status)
}

// Input returns the serialized input payload.
func (m *AutomationRun) Input() types.JSONRaw {
	raw, _ := m.GetRaw("input").(types.JSONRaw)
	return raw
}

// SetInput updates the serialized input payload.
func (m *AutomationRun) SetInput(input types.JSONRaw) {
	m.Set("input", input)
}

// StepResults returns the serialized step results payload.
func (m *AutomationRun) StepResults() types.JSONRaw {
	raw, _ := m.GetRaw("stepResults").(types.JSONRaw)
	return raw
}

// SetStepResults updates the serialized step results payload.
func (m *AutomationRun) SetStepResults(stepResults types.JSONRaw) {
	m.Set("stepResults", stepResults)
}

// Error returns the optional error message.
func (m *AutomationRun) Error() string {
	return m.GetString("error")
}

// SetError updates the optional error message.
func (m *AutomationRun) SetError(errMsg string) {
	m.Set("error", errMsg)
}

// ErrorStepIndex returns the index of the failed step, if any.
func (m *AutomationRun) ErrorStepIndex() int {
	return m.GetInt("errorStepIndex")
}

// HasErrorStepIndex checks whether the run has a stored failed step index.
func (m *AutomationRun) HasErrorStepIndex() bool {
	return m.GetInt("errorStepIndex") >= 0
}

// SetErrorStepIndex updates the failed step index.
func (m *AutomationRun) SetErrorStepIndex(index int) {
	m.Set("errorStepIndex", index)
}

// ClearErrorStepIndex clears the failed step index.
func (m *AutomationRun) ClearErrorStepIndex() {
	m.Set("errorStepIndex", automationRunErrorStepIndexUnset)
}

// ParentRunId returns the parent automation run id, if this run was triggered by another automation.
func (m *AutomationRun) ParentRunId() string {
	return m.GetString("parentRunId")
}

// SetParentRunId updates the parent automation run id.
func (m *AutomationRun) SetParentRunId(runId string) {
	m.Set("parentRunId", runId)
}

// Depth returns the run recursion depth.
func (m *AutomationRun) Depth() int {
	return m.GetInt("depth")
}

// SetDepth updates the run recursion depth.
func (m *AutomationRun) SetDepth(depth int) {
	m.Set("depth", depth)
}

// DedupeKey returns the optional run dedupe key.
func (m *AutomationRun) DedupeKey() string {
	return m.GetString("dedupeKey")
}

// SetDedupeKey updates the optional run dedupe key.
func (m *AutomationRun) SetDedupeKey(key string) {
	m.Set("dedupeKey", key)
}

// PolicyDecision returns the policy decision audit payload.
func (m *AutomationRun) PolicyDecision() types.JSONRaw {
	raw, _ := m.GetRaw("policyDecision").(types.JSONRaw)
	return raw
}

// SetPolicyDecision updates the policy decision audit payload.
func (m *AutomationRun) SetPolicyDecision(decision types.JSONRaw) {
	m.Set("policyDecision", decision)
}

// WorkflowVersionRef returns the published workflow version used for the run.
func (m *AutomationRun) WorkflowVersionRef() string {
	return m.GetString("workflowVersionRef")
}

// SetWorkflowVersionRef updates the published workflow version used for the run.
func (m *AutomationRun) SetWorkflowVersionRef(id string) {
	m.Set("workflowVersionRef", id)
}

// WorkflowVersionSnapshot returns the immutable automation snapshot used for the run.
func (m *AutomationRun) WorkflowVersionSnapshot() types.JSONRaw {
	raw, _ := m.GetRaw("workflowVersionSnapshot").(types.JSONRaw)
	return raw
}

// SetWorkflowVersionSnapshot updates the immutable automation snapshot used for the run.
func (m *AutomationRun) SetWorkflowVersionSnapshot(snapshot types.JSONRaw) {
	m.Set("workflowVersionSnapshot", snapshot)
}

// Started returns the run start timestamp.
func (m *AutomationRun) Started() types.DateTime {
	return m.GetDateTime("started")
}

// Finished returns the run finish timestamp.
func (m *AutomationRun) Finished() types.DateTime {
	return m.GetDateTime("finished")
}

// Created returns the "created" record field value.
func (m *AutomationRun) Created() types.DateTime {
	return m.GetDateTime("created")
}

// Updated returns the "updated" record field value.
func (m *AutomationRun) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
