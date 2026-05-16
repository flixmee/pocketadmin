package core

import (
	"fmt"
	"strings"
)

const StoreKeyAutomationAIProvider = "pbAppAutomationAIProvider"

type AutomationAIRequest struct {
	StepType string         `json:"stepType"`
	Model    string         `json:"model,omitempty"`
	Input    any            `json:"input,omitempty"`
	Schema   map[string]any `json:"schema,omitempty"`
	Labels   []string       `json:"labels,omitempty"`
}

type AutomationAIResponse struct {
	Output     any            `json:"output"`
	Model      string         `json:"model,omitempty"`
	TokenUsage map[string]int `json:"tokenUsage,omitempty"`
}

type AutomationAIProvider interface {
	RunAutomationAI(req AutomationAIRequest) (AutomationAIResponse, error)
}

type deterministicAutomationAIProvider struct{}

func (deterministicAutomationAIProvider) RunAutomationAI(req AutomationAIRequest) (AutomationAIResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "deterministic"
	}

	var output any
	switch req.StepType {
	case AutomationStepAIExtract:
		output = deterministicAIObject(req.Schema, req.Input)
	case AutomationStepAIClassify:
		if len(req.Labels) > 0 {
			output = map[string]any{"label": req.Labels[0], "confidence": 1}
		} else {
			output = map[string]any{"label": "unclassified", "confidence": 1}
		}
	case AutomationStepAIGenerate:
		output = map[string]any{"text": fmt.Sprint(req.Input)}
	case AutomationStepAISummarize:
		text := fmt.Sprint(req.Input)
		if len(text) > 160 {
			text = text[:160]
		}
		output = map[string]any{"summary": text}
	default:
		return AutomationAIResponse{}, fmt.Errorf("unsupported AI step type %q", req.StepType)
	}

	return AutomationAIResponse{
		Output: output,
		Model:  model,
		TokenUsage: map[string]int{
			"input":  len(fmt.Sprint(req.Input)),
			"output": len(fmt.Sprint(output)),
		},
	}, nil
}

func getAutomationAIProvider(app App) AutomationAIProvider {
	if provider, ok := app.Store().Get(StoreKeyAutomationAIProvider).(AutomationAIProvider); ok && provider != nil {
		return provider
	}

	return deterministicAutomationAIProvider{}
}

func executeAutomationAIStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	stepType := strings.TrimSpace(toString(step["type"]))

	input, err := renderAutomationTemplateValue(step["input"], ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	schema, err := automationAIObjectField(step, "schema")
	if err != nil {
		return nil, err
	}

	labels, err := automationAIStringListField(step, "labels")
	if err != nil {
		return nil, err
	}

	response, err := getAutomationAIProvider(ctx.App).RunAutomationAI(AutomationAIRequest{
		StepType: stepType,
		Model:    strings.TrimSpace(toString(step["model"])),
		Input:    input,
		Schema:   schema,
		Labels:   labels,
	})
	if err != nil {
		return nil, err
	}

	if err := validateAutomationAIOutput(response.Output, schema); err != nil {
		return nil, err
	}

	return map[string]any{
		"model":      response.Model,
		"output":     response.Output,
		"tokenUsage": response.TokenUsage,
	}, nil
}

func automationAIObjectField(step map[string]any, key string) (map[string]any, error) {
	if step[key] == nil {
		return nil, nil
	}
	value, ok := step[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("AI step %s must be a JSON object", key)
	}
	return value, nil
}

func automationAIStringListField(step map[string]any, key string) ([]string, error) {
	if step[key] == nil {
		return nil, nil
	}
	raw, ok := step[key].([]any)
	if !ok {
		return nil, fmt.Errorf("AI step %s must be a JSON array", key)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result, nil
}

func deterministicAIObject(schema map[string]any, input any) map[string]any {
	result := map[string]any{}
	properties, _ := schema["properties"].(map[string]any)
	for key, raw := range properties {
		prop, _ := raw.(map[string]any)
		switch prop["type"] {
		case "number", "integer":
			result[key] = 0
		case "boolean":
			result[key] = false
		case "array":
			result[key] = []any{}
		case "object":
			result[key] = map[string]any{}
		default:
			result[key] = fmt.Sprint(input)
		}
	}
	return result
}

func validateAutomationAIOutput(output any, schema map[string]any) error {
	if len(schema) == 0 {
		return nil
	}
	required, _ := schema["required"].([]any)
	if len(required) == 0 {
		return nil
	}
	outputMap, ok := output.(map[string]any)
	if !ok {
		return fmt.Errorf("AI output must be an object")
	}
	for _, raw := range required {
		key := strings.TrimSpace(toString(raw))
		if key == "" {
			continue
		}
		if _, ok := outputMap[key]; !ok {
			return fmt.Errorf("AI output is missing required field %q", key)
		}
	}
	return nil
}
