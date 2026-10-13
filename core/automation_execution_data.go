package core

import (
	"errors"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
)

// AutomationStepExecutionData sets named values that a manual automation
// returns to whoever ran it (the manual run API or a calling automation).
const AutomationStepExecutionData = "execution.data"

func executeAutomationExecutionDataStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	raw, ok := automationExecutionDataConfig(step)
	if !ok {
		return nil, errors.New("execution data step data must be a JSON object")
	}

	rendered, err := renderAutomationTemplateValue(raw, ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	data, ok := rendered.(map[string]any)
	if !ok {
		return nil, errors.New("execution data step data must be a JSON object")
	}

	return data, nil
}

func validateAutomationExecutionDataStep(automationRecord *Record, step map[string]any) error {
	if strings.TrimSpace(automationRecord.GetString("triggerType")) != AutomationTriggerManual {
		return validation.NewError("validation_invalid_automation_execution_data", "Execution data steps require a manual trigger.")
	}

	data, ok := automationExecutionDataConfig(step)
	if !ok {
		return validation.NewError("validation_invalid_automation_execution_data", "Execution data must be a JSON object.")
	}
	if len(data) == 0 {
		return validation.NewError("validation_invalid_automation_execution_data", "Execution data requires at least one field.")
	}
	for key := range data {
		if strings.TrimSpace(key) == "" {
			return validation.NewError("validation_invalid_automation_execution_data", "Execution data field names can't be empty.")
		}
	}

	return nil
}

func automationExecutionDataConfig(step map[string]any) (map[string]any, bool) {
	data, ok := step["data"].(map[string]any)
	return data, ok
}

// automationExecutionData merges the outputs of all successful execution data
// steps of the run, in execution order (later steps override earlier keys).
//
// It returns nil if the run has no execution data steps.
func automationExecutionData(ctx *automationExecutionContext) map[string]any {
	steps, _ := ctx.TemplateData["steps"].([]any)

	var result map[string]any
	for _, item := range steps {
		entry, _ := item.(map[string]any)
		if entry == nil || toString(entry["type"]) != AutomationStepExecutionData || toString(entry["status"]) != automationStepStatusSuccess {
			continue
		}

		output, _ := entry["output"].(map[string]any)
		if result == nil {
			result = make(map[string]any, len(output))
		}
		for key, value := range output {
			result[key] = value
		}
	}

	return result
}

func automationExecutionDataStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepExecutionData,
		Label:    "Execution data",
		Category: "output",
		InputSchema: objectSchema(map[string]any{
			"type": constStringSchema(AutomationStepExecutionData),
			"data": objectSchema(nil),
		}),
		OutputSchema: objectSchema(nil),
	}
}
