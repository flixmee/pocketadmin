package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	StoreKeyAutomationHTTPDoer = "pbAppAutomationHTTPDoer"

	AutomationMaxSteps              = 100
	AutomationMaxTemplateStringSize = 256 * 1024
	AutomationCodeStepTimeout       = 2 * time.Second

	automationStepStatusSuccess = "success"
	automationStepStatusFailed  = "failed"
	automationStepStatusStopped = "stopped"

	automationConditionOpEq            = "eq"
	automationConditionOpNeq           = "neq"
	automationConditionOpIn            = "in"
	automationConditionOpExists        = "exists"
	automationConditionOpEmpty         = "empty"
	automationConditionOpNotEmpty      = "notEmpty"
	automationConditionOpStartsWith    = "startsWith"
	automationConditionOpEndsWith      = "endsWith"
	automationConditionOpNotStartsWith = "notStartsWith"
	automationConditionOpNotEndsWith   = "notEndsWith"
	automationConditionOpContains      = "contains"

	automationTemplateAppKey            = "__automationApp"
	automationTemplateRecordModelKey    = "__automationRecordModel"
	automationTemplateOriginalRecordKey = "__automationOriginalRecordModel"
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
			"request":                           payload.Request,
			"i18n":                              payload.I18n,
			"record":                            payload.Record,
			"recordOriginal":                    payload.RecordOriginal,
			"automation":                        automationTemplateRecordData(automation.Record),
			"run":                               automationTemplateRecordData(run.Record),
			"steps":                             []any{},
			automationTemplateAppKey:            app,
			automationTemplateRecordModelKey:    payload.triggerRecord,
			automationTemplateOriginalRecordKey: payload.originalRecord,
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

	if ctx.DryRun && stepType != AutomationStepCondition && stepType != AutomationStepCode {
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
	case AutomationStepCode:
		output, err := executeAutomationCodeStep(ctx, step)
		return automationStepStatusSuccess, output, err
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
	case AutomationStepCode:
		return executeAutomationCodeStep(ctx, step)
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

func executeAutomationCodeStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	code := strings.TrimSpace(toString(step["code"]))
	if code == "" {
		return nil, fmt.Errorf("code step is missing JavaScript code")
	}

	vm := goja.New()
	jsContext := automationTemplateJSContext(ctx.TemplateData)
	for key, value := range jsContext {
		if strings.HasPrefix(key, "__") {
			continue
		}
		if err := vm.Set(key, value); err != nil {
			return nil, fmt.Errorf("failed to initialize code step root %q: %w", key, err)
		}
	}

	output := map[string]any{}
	if err := vm.Set("output", output); err != nil {
		return nil, fmt.Errorf("failed to initialize code step output: %w", err)
	}
	if err := vm.Set("__args", []any{automationCodeStepContextArg(jsContext)}); err != nil {
		return nil, fmt.Errorf("failed to initialize code step arguments: %w", err)
	}

	timeout := time.AfterFunc(AutomationCodeStepTimeout, func() {
		vm.Interrupt("automation code step timed out")
	})
	defer timeout.Stop()

	result, err := runAutomationCodeStepScript(vm, code)
	if err != nil {
		return nil, fmt.Errorf("failed to execute automation code step: %w", err)
	}
	if goja.IsUndefined(result) {
		return output, nil
	}

	return normalizeAutomationCodeStepOutput(result)
}

func automationCodeStepContextArg(jsContext map[string]any) map[string]any {
	arg := map[string]any{}
	for key, value := range jsContext {
		if strings.HasPrefix(key, "__") {
			continue
		}
		arg[key] = value
	}
	return arg
}

func runAutomationCodeStepScript(vm *goja.Runtime, code string) (goja.Value, error) {
	if automationCodeStepLooksCallable(code) {
		result, err := vm.RunScript("automation-code-step.js", "("+code+").apply(undefined, __args)")
		if err == nil {
			return result, nil
		}
	}

	return vm.RunScript("automation-code-step.js", `(function() {
`+code+`
if (typeof run === "function") {
	return run.apply(undefined, __args);
}
if (typeof main === "function") {
	return main.apply(undefined, __args);
}
if (typeof execute === "function") {
	return execute.apply(undefined, __args);
}
if (typeof handler === "function") {
	return handler.apply(undefined, __args);
}
})()`)
}

func automationCodeStepLooksCallable(code string) bool {
	trimmed := strings.TrimSpace(code)
	if strings.HasPrefix(trimmed, "function") {
		return true
	}
	if arrowIndex := strings.Index(trimmed, "=>"); arrowIndex > 0 {
		left := strings.TrimSpace(trimmed[:arrowIndex])
		return strings.HasPrefix(left, "(") || strings.HasPrefix(left, "async ") || automationCodeStepLooksIdentifier(left)
	}
	return false
}

func automationCodeStepLooksIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if r == '_' || r == '$' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') {
			continue
		}
		if i > 0 && '0' <= r && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func normalizeAutomationCodeStepOutput(result goja.Value) (map[string]any, error) {
	exported := result.Export()
	if exported == nil {
		return map[string]any{}, nil
	}

	raw, err := json.Marshal(exported)
	if err != nil {
		return nil, fmt.Errorf("automation code step must return JSON-compatible object data: %w", err)
	}

	normalized := map[string]any{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, fmt.Errorf("automation code step must return an object")
	}

	return normalized, nil
}

func executeAutomationConditionStep(ctx *automationExecutionContext, step map[string]any) (string, any, error) {
	conditionRules := automationConditionRules(step)
	if len(conditionRules) > 1 {
		matchMode := strings.TrimSpace(toString(step["match"]))
		if matchMode != "or" {
			matchMode = "and"
		}

		output := map[string]any{
			"match":      matchMode,
			"conditions": []any{},
		}

		matched := matchMode == "and"
		conditionOutputs := make([]any, 0, len(conditionRules))
		for _, condition := range conditionRules {
			conditionOutput, conditionMatched, err := evaluateAutomationConditionRule(ctx, condition)
			if err != nil {
				return automationStepStatusFailed, output, err
			}

			conditionOutputs = append(conditionOutputs, conditionOutput)
			if matchMode == "or" {
				matched = matched || conditionMatched
			} else {
				matched = matched && conditionMatched
			}
		}

		output["conditions"] = conditionOutputs
		output["matched"] = matched
		if matched {
			return automationStepStatusSuccess, output, nil
		}

		return automationStepStatusStopped, output, nil
	}

	output, matched, err := evaluateAutomationConditionRule(ctx, conditionRules[0])
	if err != nil {
		return automationStepStatusFailed, output, err
	}
	if matched {
		return automationStepStatusSuccess, output, nil
	}

	return automationStepStatusStopped, output, nil
}

func automationConditionRules(step map[string]any) []map[string]any {
	result := []map[string]any{}

	if conditions, ok := step["conditions"].([]any); ok {
		for _, item := range conditions {
			if condition, ok := item.(map[string]any); ok {
				result = append(result, condition)
			}
		}
	}
	if conditions, ok := step["conditions"].([]map[string]any); ok {
		result = append(result, conditions...)
	}

	if len(result) == 0 {
		result = append(result, step)
	}

	return result
}

func evaluateAutomationConditionRule(ctx *automationExecutionContext, step map[string]any) (map[string]any, bool, error) {
	path := strings.TrimSpace(toString(step["path"]))
	if path == "" {
		path = strings.TrimSpace(toString(step["field"]))
	}
	if path == "" {
		return nil, false, fmt.Errorf("condition step is missing path")
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
			return output, true, nil
		}

		output["matched"] = false
		return output, false, nil
	case automationConditionOpEmpty:
		matched := !found || isEmptyAutomationValue(actual)
		output["matched"] = matched
		return output, matched, nil
	case automationConditionOpNotEmpty:
		matched := found && !isEmptyAutomationValue(actual)
		output["matched"] = matched
		return output, matched, nil
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
			return output, false, nil
		}

		expected, err := renderAutomationTemplateValue(step["value"], ctx.TemplateData)
		if err != nil {
			return output, false, err
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

		return output, matched, nil
	default:
		return output, false, fmt.Errorf("unsupported condition operator %q", op)
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
