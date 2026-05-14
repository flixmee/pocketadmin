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
	App             App
	Automation      *Automation
	Run             *AutomationRun
	Payload         automationTriggerPayload
	TriggerRecord   *Record
	OriginalRecord  *Record
	TemplateData    map[string]any
	WebhookResponse *AutomationWebhookResponse
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
				"i18n":           payload.I18n,
			},
			"request":        payload.Request,
			"i18n":           payload.I18n,
			"record":         payload.Record,
			"recordOriginal": payload.RecordOriginal,
			"automation":     automationTemplateRecordData(automation.Record),
			"run":            automationTemplateRecordData(run.Record),
			"steps":          []any{},
		},
	}
}

func (ctx *automationExecutionContext) appendStepTemplateResult(result automationStepResult) {
	entry := map[string]any{
		"index":      result.Index,
		"type":       result.Type,
		"status":     result.Status,
		"started":    result.Started,
		"finished":   result.Finished,
		"durationMs": result.DurationMs,
	}
	if result.Output != nil {
		entry["output"] = result.Output
	}
	if result.Error != "" {
		entry["error"] = result.Error
	}

	steps, _ := ctx.TemplateData["steps"].([]any)
	steps = append(steps, entry)
	ctx.TemplateData["steps"] = steps
	ctx.TemplateData["prevStep"] = entry
}

func executeAutomationStep(ctx *automationExecutionContext, step map[string]any) (string, any, error) {
	stepType := strings.TrimSpace(toString(step["type"]))

	switch stepType {
	case AutomationStepCondition:
		return executeAutomationConditionStep(ctx, step)
	case AutomationStepHTTP:
		output, err := executeAutomationHTTPStep(ctx, step)
		return automationStepStatusSuccess, output, err
	case AutomationStepMailSend:
		output, err := executeAutomationMailStep(ctx, step)
		return automationStepStatusSuccess, output, err
	case AutomationStepRecordCreate:
		output, err := executeAutomationRecordCreateStep(ctx, step)
		return automationStepStatusSuccess, output, err
	case AutomationStepRecordUpdate:
		output, err := executeAutomationRecordUpdateStep(ctx, step)
		return automationStepStatusSuccess, output, err
	case AutomationStepRecordDelete:
		output, err := executeAutomationRecordDeleteStep(ctx, step)
		return automationStepStatusSuccess, output, err
	case AutomationStepResponse:
		output, err := executeAutomationResponseStep(ctx, step)
		return automationStepStatusSuccess, output, err
	default:
		return automationStepStatusFailed, nil, fmt.Errorf("unsupported automation step type %q", stepType)
	}
}

func executeAutomationConditionStep(ctx *automationExecutionContext, step map[string]any) (string, any, error) {
	path := strings.TrimSpace(toString(step["path"]))
	if path == "" {
		path = strings.TrimSpace(toString(step["field"]))
	}
	if path == "" {
		return automationStepStatusFailed, nil, fmt.Errorf("condition step is missing path")
	}

	op := strings.TrimSpace(toString(step["op"]))
	actual, found := resolveAutomationTemplatePath(ctx.TemplateData, path)
	output := map[string]any{
		"path":  path,
		"op":    op,
		"found": found,
	}
	if found {
		output["actual"] = actual
	}

	switch op {
	case automationConditionOpExists:
		if found && !isNilAutomationValue(actual) {
			output["matched"] = true
			return automationStepStatusSuccess, output, nil
		}

		output["matched"] = false
		return automationStepStatusStopped, output, nil
	case automationConditionOpEq,
		automationConditionOpNeq,
		automationConditionOpIn,
		automationConditionOpStartsWith,
		automationConditionOpEndsWith,
		automationConditionOpNotStartsWith,
		automationConditionOpNotEndsWith,
		automationConditionOpContains:
		if !found {
			output["matched"] = false
			return automationStepStatusStopped, output, nil
		}

		expected, err := renderAutomationTemplateValue(step["value"], ctx.TemplateData)
		if err != nil {
			return automationStepStatusFailed, output, err
		}
		output["expected"] = expected

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
		output["matched"] = matched

		if matched {
			return automationStepStatusSuccess, output, nil
		}

		return automationStepStatusStopped, output, nil
	default:
		return automationStepStatusFailed, output, fmt.Errorf("unsupported condition operator %q", op)
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
