package core

import (
	"context"
	"errors"

	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameAutomations = "_automations"

	AutomationTriggerRecordCreate       = "record.create"
	AutomationTriggerRecordUpdate       = "record.update"
	AutomationTriggerRecordDelete       = "record.delete"
	AutomationTriggerRecordBeforeCreate = "record.beforeCreate"
	AutomationTriggerRecordBeforeUpdate = "record.beforeUpdate"
	AutomationTriggerScheduleCron       = "schedule.cron"
	AutomationTriggerWebhook            = "webhook"
	AutomationTriggerTelegramMessage    = "telegram.message"
	AutomationTriggerManual             = "manual"
	AutomationTriggerI18nMissing        = "i18n.translation_missing"
	AutomationTriggerI18nPublished      = "i18n.locale_published"
	AutomationTriggerI18nUpdated        = "i18n.translation_updated"
	AutomationTriggerI18nAIFinished     = "i18n.ai_translation_finished"

	AutomationStepCondition    = "condition"
	AutomationStepCode         = "code"
	AutomationStepHTTP         = "http"
	AutomationStepMailSend     = "mail.send"
	AutomationStepTelegramSend = "telegram.send"
	AutomationStepRecordCreate = "record.create"
	AutomationStepRecordUpdate = "record.update"
	AutomationStepRecordDelete = "record.delete"
	AutomationStepResponse     = "response"
	AutomationStepCapability   = "capability"
	AutomationStepWaitDelay    = "wait.delay"
	AutomationStepWaitWebhook  = "wait.webhook"
	AutomationStepWaitEvent    = "wait.event"
	AutomationStepWaitApproval = "wait.approval"
	AutomationStepAIExtract    = "ai.extract"
	AutomationStepAIClassify   = "ai.classify"
	AutomationStepAIGenerate   = "ai.generate"
	AutomationStepAISummarize  = "ai.summarize"
)

var (
	_ Model        = (*Automation)(nil)
	_ PreValidator = (*Automation)(nil)
	_ RecordProxy  = (*Automation)(nil)
)

// Automation defines a Record proxy for working with the automations collection.
type Automation struct {
	*Record
}

// NewAutomation instantiates and returns a new blank *Automation model.
func NewAutomation(app App) *Automation {
	m := &Automation{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameAutomations)
	if err != nil {
		// this is just to make tests easier since automations is a system collection and it is expected to be always accessible
		// (note: the loaded record is further checked on Automation.PreValidate())
		c = NewBaseCollection("@__invalid_automations__")
	}

	m.Record = NewRecord(c)

	return m
}

// PreValidate implements the [PreValidator] interface and checks
// whether the proxy is properly loaded.
func (m *Automation) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameAutomations {
		return errors.New("missing or invalid Automation ProxyRecord")
	}

	return nil
}

// ProxyRecord returns the proxied Record model.
func (m *Automation) ProxyRecord() *Record {
	return m.Record
}

// SetProxyRecord loads the specified record model into the current proxy.
func (m *Automation) SetProxyRecord(record *Record) {
	m.Record = record
}

// Name returns the automation name.
func (m *Automation) Name() string {
	return m.GetString("name")
}

// SetName updates the automation name.
func (m *Automation) SetName(name string) {
	m.Set("name", name)
}

// Tag returns the optional automation grouping tag.
func (m *Automation) Tag() string {
	return m.GetString("tag")
}

// SetTag updates the optional automation grouping tag.
func (m *Automation) SetTag(tag string) {
	m.Set("tag", tag)
}

// Active returns whether the automation is enabled.
func (m *Automation) Active() bool {
	return m.GetBool("active")
}

// SetActive updates the automation enabled state.
func (m *Automation) SetActive(active bool) {
	m.Set("active", active)
}

// NotifyOnCompletion returns whether completed runs should notify admins.
func (m *Automation) NotifyOnCompletion() bool {
	return m.GetBool("notifyOnCompletion")
}

// SetNotifyOnCompletion updates the completed run admin notification setting.
func (m *Automation) SetNotifyOnCompletion(notify bool) {
	m.Set("notifyOnCompletion", notify)
}

// TriggerType returns the configured trigger type.
func (m *Automation) TriggerType() string {
	return m.GetString("triggerType")
}

// SetTriggerType updates the configured trigger type.
func (m *Automation) SetTriggerType(triggerType string) {
	m.Set("triggerType", triggerType)
}

// CollectionRef returns the trigger collection reference.
func (m *Automation) CollectionRef() string {
	return m.GetString("collectionRef")
}

// SetCollectionRef updates the trigger collection reference.
func (m *Automation) SetCollectionRef(collectionId string) {
	m.Set("collectionRef", collectionId)
}

// CronExpr returns the schedule cron expression.
func (m *Automation) CronExpr() string {
	return m.GetString("cronExpr")
}

// SetCronExpr updates the schedule cron expression.
func (m *Automation) SetCronExpr(cronExpr string) {
	m.Set("cronExpr", cronExpr)
}

// Steps returns the serialized steps JSON payload.
func (m *Automation) Steps() types.JSONRaw {
	raw, _ := m.GetRaw("steps").(types.JSONRaw)
	return raw
}

// SetSteps updates the serialized steps JSON payload.
func (m *Automation) SetSteps(steps types.JSONRaw) {
	m.Set("steps", steps)
}

// Notes returns the optional notes field value.
func (m *Automation) Notes() string {
	return m.GetString("notes")
}

// SetNotes updates the optional notes field value.
func (m *Automation) SetNotes(notes string) {
	m.Set("notes", notes)
}

// LastRunAt returns the last run timestamp.
func (m *Automation) LastRunAt() types.DateTime {
	return m.GetDateTime("lastRunAt")
}

// LastRunStatus returns the last run status.
func (m *Automation) LastRunStatus() string {
	return m.GetString("lastRunStatus")
}

// Created returns the "created" record field value.
func (m *Automation) Created() types.DateTime {
	return m.GetDateTime("created")
}

// Updated returns the "updated" record field value.
func (m *Automation) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
