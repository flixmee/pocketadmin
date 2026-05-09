package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	StoreKeyAutomationHTTPDoer = "pbAppAutomationHTTPDoer"

	automationStepStatusSuccess = "success"
	automationStepStatusFailed  = "failed"
	automationStepStatusStopped = "stopped"

	automationConditionOpEq            = "eq"
	automationConditionOpNeq           = "neq"
	automationConditionOpIn            = "in"
	automationConditionOpExists        = "exists"
	automationConditionOpStartsWith    = "startsWith"
	automationConditionOpEndsWith      = "endsWith"
	automationConditionOpNotStartsWith = "notStartsWith"
	automationConditionOpNotEndsWith   = "notEndsWith"
	automationConditionOpContains      = "contains"
)

type automationExecutionContext struct {
	App            App
	Automation     *Automation
	Run            *AutomationRun
	Payload        automationTriggerPayload
	TriggerRecord  *Record
	OriginalRecord *Record
	TemplateData   map[string]any
}

func newAutomationExecutionContext(app App, automation *Automation, run *AutomationRun, payload automationTriggerPayload) *automationExecutionContext {
	return &automationExecutionContext{
		App:            app,
		Automation:     automation,
		Run:            run,
		Payload:        payload,
		TriggerRecord:  payload.triggerRecord,
		OriginalRecord: payload.originalRecord,
		TemplateData: map[string]any{
			"trigger": map[string]any{
				"type":           payload.TriggerType,
				"collectionId":   payload.CollectionId,
				"collectionName": payload.CollectionName,
				"request":        payload.Request,
			},
			"request":        payload.Request,
			"record":         payload.Record,
			"recordOriginal": payload.RecordOriginal,
			"automation":     automationTemplateRecordData(automation.Record),
			"run":            automationTemplateRecordData(run.Record),
		},
	}
}

func executeAutomationStep(ctx *automationExecutionContext, step map[string]any) (string, error) {
	stepType := strings.TrimSpace(toString(step["type"]))

	switch stepType {
	case AutomationStepCondition:
		return executeAutomationConditionStep(ctx, step)
	case AutomationStepHTTP:
		return automationStepStatusSuccess, executeAutomationHTTPStep(ctx, step)
	case AutomationStepMailSend:
		return automationStepStatusSuccess, executeAutomationMailStep(ctx, step)
	case AutomationStepRecordCreate:
		return automationStepStatusSuccess, executeAutomationRecordCreateStep(ctx, step)
	case AutomationStepRecordUpdate:
		return automationStepStatusSuccess, executeAutomationRecordUpdateStep(ctx, step)
	case AutomationStepRecordDelete:
		return automationStepStatusSuccess, executeAutomationRecordDeleteStep(ctx, step)
	default:
		return automationStepStatusFailed, fmt.Errorf("unsupported automation step type %q", stepType)
	}
}

func executeAutomationConditionStep(ctx *automationExecutionContext, step map[string]any) (string, error) {
	path := strings.TrimSpace(toString(step["path"]))
	if path == "" {
		path = strings.TrimSpace(toString(step["field"]))
	}
	if path == "" {
		return automationStepStatusFailed, fmt.Errorf("condition step is missing path")
	}

	op := strings.TrimSpace(toString(step["op"]))
	actual, found := resolveAutomationTemplatePath(ctx.TemplateData, path)

	switch op {
	case automationConditionOpExists:
		if found && !isNilAutomationValue(actual) {
			return automationStepStatusSuccess, nil
		}

		return automationStepStatusStopped, nil
	case automationConditionOpEq,
		automationConditionOpNeq,
		automationConditionOpIn,
		automationConditionOpStartsWith,
		automationConditionOpEndsWith,
		automationConditionOpNotStartsWith,
		automationConditionOpNotEndsWith,
		automationConditionOpContains:
		if !found {
			return automationStepStatusStopped, nil
		}

		expected, err := renderAutomationTemplateValue(step["value"], ctx.TemplateData)
		if err != nil {
			return automationStepStatusFailed, err
		}

		matched := false
		switch op {
		case automationConditionOpEq:
			matched = automationValuesEqual(actual, expected)
		case automationConditionOpNeq:
			matched = !automationValuesEqual(actual, expected)
		case automationConditionOpIn:
			matched = automationValueIn(actual, expected)
		case automationConditionOpStartsWith:
			matched = automationValueStartsWith(actual, expected)
		case automationConditionOpEndsWith:
			matched = automationValueEndsWith(actual, expected)
		case automationConditionOpNotStartsWith:
			matched = !automationValueStartsWith(actual, expected)
		case automationConditionOpNotEndsWith:
			matched = !automationValueEndsWith(actual, expected)
		case automationConditionOpContains:
			matched = automationValueContains(actual, expected)
		}

		if matched {
			return automationStepStatusSuccess, nil
		}

		return automationStepStatusStopped, nil
	default:
		return automationStepStatusFailed, fmt.Errorf("unsupported condition operator %q", op)
	}
}

func decodeAutomationSteps(raw string) ([]map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return nil, nil
	}

	steps := []map[string]any{}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return nil, fmt.Errorf("failed to decode automation steps: %w", err)
	}

	return steps, nil
}

func automationTemplateRecordData(record *Record) map[string]any {
	if record == nil {
		return map[string]any{}
	}

	data := record.FieldsData()
	data["id"] = record.Id
	data["created"] = record.Get("created")
	data["updated"] = record.Get("updated")

	return data
}
