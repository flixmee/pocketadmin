package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionNameAutomationEvents = "_automationEvents"

var (
	_ Model        = (*AutomationEventRecord)(nil)
	_ PreValidator = (*AutomationEventRecord)(nil)
	_ RecordProxy  = (*AutomationEventRecord)(nil)
)

type AutomationEventRecord struct {
	*Record
}

type AutomationEventEnvelope struct {
	Id            string         `json:"id,omitempty"`
	Name          string         `json:"name"`
	Source        string         `json:"source,omitempty"`
	Subject       string         `json:"subject,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
	Occurred      types.DateTime `json:"occurred"`
	CorrelationId string         `json:"correlationId,omitempty"`
	CausationId   string         `json:"causationId,omitempty"`
}

func NewAutomationEventRecord(app App) *AutomationEventRecord {
	m := &AutomationEventRecord{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameAutomationEvents)
	if err != nil {
		c = NewBaseCollection("@__invalid_automation_events__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *AutomationEventRecord) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameAutomationEvents {
		return errors.New("missing or invalid AutomationEventRecord ProxyRecord")
	}

	return nil
}

func (m *AutomationEventRecord) ProxyRecord() *Record {
	return m.Record
}

func (m *AutomationEventRecord) SetProxyRecord(record *Record) {
	m.Record = record
}
