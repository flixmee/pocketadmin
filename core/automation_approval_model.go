package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameApprovals = "_approvals"

	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)

var (
	_ Model        = (*Approval)(nil)
	_ PreValidator = (*Approval)(nil)
	_ RecordProxy  = (*Approval)(nil)
)

// Approval defines a pending or resolved human workflow decision.
type Approval struct {
	*Record
}

func NewApproval(app App) *Approval {
	m := &Approval{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameApprovals)
	if err != nil {
		c = NewBaseCollection("@__invalid_approvals__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *Approval) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameApprovals {
		return errors.New("missing or invalid Approval ProxyRecord")
	}

	return nil
}

func (m *Approval) ProxyRecord() *Record {
	return m.Record
}

func (m *Approval) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Approval) WorkflowStateRef() string {
	return m.GetString("workflowStateRef")
}

func (m *Approval) SetWorkflowStateRef(id string) {
	m.Set("workflowStateRef", id)
}

func (m *Approval) AutomationRef() string {
	return m.GetString("automationRef")
}

func (m *Approval) SetAutomationRef(id string) {
	m.Set("automationRef", id)
}

func (m *Approval) RunRef() string {
	return m.GetString("runRef")
}

func (m *Approval) SetRunRef(id string) {
	m.Set("runRef", id)
}

func (m *Approval) StepIndex() int {
	return m.GetInt("stepIndex")
}

func (m *Approval) SetStepIndex(index int) {
	m.Set("stepIndex", index)
}

func (m *Approval) Assignee() string {
	return m.GetString("assignee")
}

func (m *Approval) SetAssignee(assignee string) {
	m.Set("assignee", assignee)
}

func (m *Approval) Role() string {
	return m.GetString("role")
}

func (m *Approval) SetRole(role string) {
	m.Set("role", role)
}

func (m *Approval) Status() string {
	return m.GetString("status")
}

func (m *Approval) SetStatus(status string) {
	m.Set("status", status)
}

func (m *Approval) Decision() string {
	return m.GetString("decision")
}

func (m *Approval) SetDecision(decision string) {
	m.Set("decision", decision)
}

func (m *Approval) Comment() string {
	return m.GetString("comment")
}

func (m *Approval) SetComment(comment string) {
	m.Set("comment", comment)
}

func (m *Approval) Resolved() types.DateTime {
	return m.GetDateTime("resolved")
}

func (m *Approval) SetResolved(resolved types.DateTime) {
	m.SetRaw("resolved", resolved)
}

func (m *Approval) Created() types.DateTime {
	return m.GetDateTime("created")
}

func (m *Approval) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
