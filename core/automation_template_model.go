package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionNameWorkflowTemplates = "_workflowTemplates"

var (
	_ Model        = (*WorkflowTemplate)(nil)
	_ PreValidator = (*WorkflowTemplate)(nil)
	_ RecordProxy  = (*WorkflowTemplate)(nil)
)

type WorkflowTemplate struct {
	*Record
}

func NewWorkflowTemplate(app App) *WorkflowTemplate {
	m := &WorkflowTemplate{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameWorkflowTemplates)
	if err != nil {
		c = NewBaseCollection("@__invalid_workflow_templates__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *WorkflowTemplate) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameWorkflowTemplates {
		return errors.New("missing or invalid WorkflowTemplate ProxyRecord")
	}

	return nil
}

func (m *WorkflowTemplate) ProxyRecord() *Record {
	return m.Record
}

func (m *WorkflowTemplate) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *WorkflowTemplate) Name() string {
	return m.GetString("name")
}

func (m *WorkflowTemplate) SetName(name string) {
	m.Set("name", name)
}

func (m *WorkflowTemplate) Description() string {
	return m.GetString("description")
}

func (m *WorkflowTemplate) SetDescription(description string) {
	m.Set("description", description)
}

func (m *WorkflowTemplate) Package() types.JSONRaw {
	raw, _ := m.GetRaw("package").(types.JSONRaw)
	return raw
}

func (m *WorkflowTemplate) SetPackage(pkg types.JSONRaw) {
	m.Set("package", pkg)
}

func (m *WorkflowTemplate) RequiredCapabilities() types.JSONRaw {
	raw, _ := m.GetRaw("requiredCapabilities").(types.JSONRaw)
	return raw
}

func (m *WorkflowTemplate) SetRequiredCapabilities(value types.JSONRaw) {
	m.Set("requiredCapabilities", value)
}

func (m *WorkflowTemplate) RequiredConnectors() types.JSONRaw {
	raw, _ := m.GetRaw("requiredConnectors").(types.JSONRaw)
	return raw
}

func (m *WorkflowTemplate) SetRequiredConnectors(value types.JSONRaw) {
	m.Set("requiredConnectors", value)
}

func (m *WorkflowTemplate) RequiredCollections() types.JSONRaw {
	raw, _ := m.GetRaw("requiredCollections").(types.JSONRaw)
	return raw
}

func (m *WorkflowTemplate) SetRequiredCollections(value types.JSONRaw) {
	m.Set("requiredCollections", value)
}

func (m *WorkflowTemplate) Active() bool {
	return m.GetBool("active")
}

func (m *WorkflowTemplate) SetActive(active bool) {
	m.Set("active", active)
}
