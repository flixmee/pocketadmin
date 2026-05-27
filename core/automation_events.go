package core

import (
	"errors"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

func (app *BaseApp) PublishAutomationEvent(event AutomationEventEnvelope) (*AutomationEventRecord, error) {
	if strings.TrimSpace(event.Name) == "" {
		return nil, errors.New("automation event name is required")
	}
	if event.Occurred.IsZero() {
		event.Occurred = types.NowDateTime()
	}
	if event.Id == "" {
		event.Id = GenerateDefaultRandomId()
	}

	record := NewAutomationEventRecord(app)
	record.SetRaw("id", event.Id)
	record.Set("name", strings.TrimSpace(event.Name))
	record.Set("source", strings.TrimSpace(event.Source))
	record.Set("subject", strings.TrimSpace(event.Subject))
	payloadRaw, err := toJSONRaw(event.Payload)
	if err != nil {
		return nil, err
	}
	record.Set("payload", payloadRaw)
	record.SetRaw("occurred", event.Occurred)
	record.Set("correlationId", strings.TrimSpace(event.CorrelationId))
	record.Set("causationId", strings.TrimSpace(event.CausationId))

	if err := app.Save(record); err != nil {
		return nil, err
	}

	return record, nil
}

func (app *BaseApp) FindAutomationEventById(id string) (*AutomationEventRecord, error) {
	result := &AutomationEventRecord{}
	err := app.RecordQuery(CollectionNameAutomationEvents).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func publishRecordAutomationEvent(app App, name string, record *Record, original *Record) {
	if automationRecordTriggersSuppressed(app) {
		return
	}
	if record == nil || record.Collection() == nil || record.Collection().System || shouldSkipAutomationTriggerCollection(record.Collection().Name) {
		return
	}
	registry, err := getAutomationRegistry(app)
	if err != nil {
		return
	}
	if registry.ByTriggerScope[name] == nil || len(registry.ByTriggerScope[name][record.Collection().Id]) == 0 {
		return
	}

	payload := newAutomationTriggerPayload(name, record, original)
	_, _ = app.PublishAutomationEvent(AutomationEventEnvelope{
		Name:    name,
		Source:  "record",
		Subject: record.Collection().Name + "/" + record.Id,
		Payload: map[string]any{
			"trigger":        payload.TriggerType,
			"collectionId":   payload.CollectionId,
			"collectionName": payload.CollectionName,
			"record":         payload.Record,
			"recordOriginal": payload.RecordOriginal,
		},
	})
}
