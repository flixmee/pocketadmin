package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	StoreKeyAutomationHTTPDoer = "pbAppAutomationHTTPDoer"

	AutomationMaxSteps              = 100
	AutomationMaxTemplateStringSize = 256 * 1024

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
	DryRun          bool
	State           *WorkflowState
	StartStepIndex  int
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

func (ctx *automationExecutionContext) appendStepTemplateResult(result AutomationStepResult) {
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

	if stepType == AutomationStepCapability {
		legacyStep, err := automationCapabilityLegacyStep(ctx.App, step)
		if err != nil {
			return automationStepStatusFailed, nil, err
		}
		if ctx.DryRun {
			output, err := previewAutomationStep(ctx, legacyStep)
			return automationStepStatusSuccess, output, err
		}
		return executeAutomationStep(ctx, legacyStep)
	}

	if ctx.DryRun && stepType != AutomationStepCondition {
		output, err := previewAutomationStep(ctx, step)
		return automationStepStatusSuccess, output, err
	}

	switch stepType {
	case AutomationStepWaitDelay, AutomationStepWaitWebhook, AutomationStepWaitEvent, AutomationStepWaitApproval:
		return automationStepStatusSuccess, map[string]any{
			"dryRun": true,
			"type":   stepType,
		}, nil
	case AutomationStepAIExtract, AutomationStepAIClassify, AutomationStepAIGenerate, AutomationStepAISummarize:
		output, err := executeAutomationAIStep(ctx, step)
		return automationStepStatusSuccess, output, err
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

func previewAutomationStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	stepType := strings.TrimSpace(toString(step["type"]))
	output := map[string]any{
		"dryRun": true,
		"type":   stepType,
	}

	switch stepType {
	case AutomationStepHTTP:
		renderedURL, err := renderAutomationTemplateString(toString(step["url"]), ctx.TemplateData)
		if err != nil {
			return nil, err
		}
		output["method"] = stringsToUpperDefault(toString(step["method"]), "GET")
		output["url"] = renderedURL
		if headers, ok := step["headers"].(map[string]any); ok {
			renderedHeaders, err := renderAutomationTemplateValue(headers, ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output["headers"] = renderedHeaders
		}
		if _, ok := step["body"]; ok {
			renderedBody, err := renderAutomationTemplateValue(step["body"], ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output["body"] = renderedBody
		}
	case AutomationStepMailSend:
		for _, key := range []string{"to", "cc", "bcc", "subject", "text", "html", "attachments"} {
			if _, ok := step[key]; !ok {
				continue
			}
			rendered, err := renderAutomationTemplateValue(step[key], ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output[key] = rendered
		}
	case AutomationStepRecordCreate, AutomationStepRecordUpdate, AutomationStepRecordDelete:
		for _, key := range []string{"collection", "id", "filter", "data"} {
			if _, ok := step[key]; !ok {
				continue
			}
			rendered, err := renderAutomationTemplateValue(step[key], ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output[key] = rendered
		}
	case AutomationStepResponse:
		for _, key := range []string{"statusCode", "headers", "body"} {
			if _, ok := step[key]; !ok {
				continue
			}
			rendered, err := renderAutomationTemplateValue(step[key], ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output[key] = rendered
		}
	case AutomationStepWaitDelay, AutomationStepWaitWebhook, AutomationStepWaitEvent, AutomationStepWaitApproval,
		AutomationStepAIExtract, AutomationStepAIClassify, AutomationStepAIGenerate, AutomationStepAISummarize:
		for _, key := range []string{"duration", "key", "assignee", "role", "model", "input", "schema", "labels"} {
			if _, ok := step[key]; !ok {
				continue
			}
			rendered, err := renderAutomationTemplateValue(step[key], ctx.TemplateData)
			if err != nil {
				return nil, err
			}
			output[key] = rendered
		}
	default:
		return nil, fmt.Errorf("unsupported automation step type %q", stepType)
	}

	return output, nil
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
