package core

import (
	"encoding/json"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

type AutomationPublishOptions struct {
	Notes       string `json:"notes,omitempty"`
	PublishedBy string `json:"publishedBy,omitempty"`
}

type automationVersionSnapshot struct {
	Id            string        `json:"id"`
	Name          string        `json:"name"`
	TriggerType   string        `json:"triggerType"`
	CollectionRef string        `json:"collectionRef,omitempty"`
	CronExpr      string        `json:"cronExpr,omitempty"`
	Steps         types.JSONRaw `json:"steps"`
	Notes         string        `json:"notes,omitempty"`
}

func (app *BaseApp) PublishAutomationVersion(automationID string, options AutomationPublishOptions) (*WorkflowVersion, error) {
	automation, err := app.FindAutomationById(automationID)
	if err != nil {
		return nil, err
	}

	version := NewWorkflowVersion(app)
	version.SetAutomationRef(automation.Id)
	version.SetVersion(nextWorkflowVersionNumber(app, automation.Id))
	version.SetStatus(WorkflowVersionStatusPublished)
	version.SetNotes(options.Notes)
	version.Set("publishedBy", strings.TrimSpace(options.PublishedBy))
	version.SetPublishedAt(types.NowDateTime())

	snapshot, err := automationSnapshotRaw(automation)
	if err != nil {
		return nil, err
	}
	version.SetSnapshot(snapshot)

	if err := app.Save(version); err != nil {
		return nil, err
	}

	return version, nil
}

func (app *BaseApp) FindLatestPublishedWorkflowVersion(automationID string) (*WorkflowVersion, error) {
	result := &WorkflowVersion{}
	err := app.RecordQuery(CollectionNameWorkflowVersions).
		AndWhere(dbx.HashExp{"automationRef": automationID, "status": WorkflowVersionStatusPublished}).
		OrderBy("version DESC").
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func nextWorkflowVersionNumber(app App, automationID string) int {
	latest := &WorkflowVersion{}
	if err := app.RecordQuery(CollectionNameWorkflowVersions).
		AndWhere(dbx.HashExp{"automationRef": automationID}).
		OrderBy("version DESC").
		Limit(1).
		One(latest); err == nil {
		return latest.Version() + 1
	}
	return 1
}

func automationSnapshotRaw(automation *Automation) (types.JSONRaw, error) {
	return toJSONRaw(automationVersionSnapshot{
		Id:            automation.Id,
		Name:          automation.Name(),
		TriggerType:   automation.TriggerType(),
		CollectionRef: automation.CollectionRef(),
		CronExpr:      automation.CronExpr(),
		Steps:         automation.Steps(),
		Notes:         automation.Notes(),
	})
}

func applyAutomationVersionSnapshot(automation *Automation, raw types.JSONRaw) (*Automation, error) {
	if strings.TrimSpace(raw.String()) == "" {
		return automation, nil
	}

	snapshot := automationVersionSnapshot{}
	if err := json.Unmarshal([]byte(raw.String()), &snapshot); err != nil {
		return nil, err
	}

	clone := &Automation{}
	clone.SetProxyRecord(automation.Record.Clone())
	if snapshot.Name != "" {
		clone.SetName(snapshot.Name)
	}
	if snapshot.TriggerType != "" {
		clone.SetTriggerType(snapshot.TriggerType)
	}
	clone.SetCollectionRef(snapshot.CollectionRef)
	clone.SetCronExpr(snapshot.CronExpr)
	if strings.TrimSpace(snapshot.Steps.String()) != "" {
		clone.SetSteps(snapshot.Steps)
	}
	clone.SetNotes(snapshot.Notes)

	return clone, nil
}
