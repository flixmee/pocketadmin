package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionNameWorkflowVersions = "_workflowVersions"

const WorkflowVersionStatusPublished = "published"

var (
	_ Model        = (*WorkflowVersion)(nil)
	_ PreValidator = (*WorkflowVersion)(nil)
	_ RecordProxy  = (*WorkflowVersion)(nil)
)

type WorkflowVersion struct {
	*Record
}

func NewWorkflowVersion(app App) *WorkflowVersion {
	m := &WorkflowVersion{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameWorkflowVersions)
	if err != nil {
		c = NewBaseCollection("@__invalid_workflow_versions__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *WorkflowVersion) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameWorkflowVersions {
		return errors.New("missing or invalid WorkflowVersion ProxyRecord")
	}

	return nil
}

func (m *WorkflowVersion) ProxyRecord() *Record {
	return m.Record
}

func (m *WorkflowVersion) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *WorkflowVersion) AutomationRef() string {
	return m.GetString("automationRef")
}

func (m *WorkflowVersion) SetAutomationRef(id string) {
	m.Set("automationRef", id)
}

func (m *WorkflowVersion) Version() int {
	return m.GetInt("version")
}

func (m *WorkflowVersion) SetVersion(version int) {
	m.Set("version", version)
}

func (m *WorkflowVersion) Status() string {
	return m.GetString("status")
}

func (m *WorkflowVersion) SetStatus(status string) {
	m.Set("status", status)
}

func (m *WorkflowVersion) Snapshot() types.JSONRaw {
	raw, _ := m.GetRaw("snapshot").(types.JSONRaw)
	return raw
}

func (m *WorkflowVersion) SetSnapshot(snapshot types.JSONRaw) {
	m.Set("snapshot", snapshot)
}

func (m *WorkflowVersion) Notes() string {
	return m.GetString("notes")
}

func (m *WorkflowVersion) SetNotes(notes string) {
	m.Set("notes", notes)
}

func (m *WorkflowVersion) PublishedAt() types.DateTime {
	return m.GetDateTime("publishedAt")
}

func (m *WorkflowVersion) SetPublishedAt(publishedAt types.DateTime) {
	m.SetRaw("publishedAt", publishedAt)
}
