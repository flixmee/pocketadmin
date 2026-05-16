package core

import (
	"fmt"
	"strings"
)

const (
	AutomationCapabilityHTTPRequest  = "http.request"
	AutomationCapabilityMailSend     = "mail.send"
	AutomationCapabilityRecordCreate = "record.create"
	AutomationCapabilityRecordUpdate = "record.update"
	AutomationCapabilityRecordDelete = "record.delete"
)

type AutomationCapabilityDefinition struct {
	Key            string           `json:"key"`
	Version        string           `json:"version"`
	Category       string           `json:"category"`
	Icon           string           `json:"icon,omitempty"`
	RuntimeHandler string           `json:"runtimeHandler"`
	Active         bool             `json:"active"`
	Schema         AutomationSchema `json:"schema"`
}

var builtInAutomationCapabilities = map[string]AutomationCapabilityDefinition{
	AutomationCapabilityHTTPRequest: {
		Key:            AutomationCapabilityHTTPRequest,
		Version:        "1.0.0",
		Category:       "integration",
		Icon:           "send",
		RuntimeHandler: AutomationStepHTTP,
		Active:         true,
		Schema:         capabilitySchemaFromStep(AutomationCapabilityHTTPRequest, "HTTP request", httpStepSchema()),
	},
	AutomationCapabilityMailSend: {
		Key:            AutomationCapabilityMailSend,
		Version:        "1.0.0",
		Category:       "communication",
		Icon:           "mail",
		RuntimeHandler: AutomationStepMailSend,
		Active:         true,
		Schema:         capabilitySchemaFromStep(AutomationCapabilityMailSend, "Send email", mailStepSchema()),
	},
	AutomationCapabilityRecordCreate: {
		Key:            AutomationCapabilityRecordCreate,
		Version:        "1.0.0",
		Category:       "record",
		Icon:           "plus",
		RuntimeHandler: AutomationStepRecordCreate,
		Active:         true,
		Schema:         capabilitySchemaFromStep(AutomationCapabilityRecordCreate, "Create record", recordWriteStepSchema(AutomationStepRecordCreate, "Create record")),
	},
	AutomationCapabilityRecordUpdate: {
		Key:            AutomationCapabilityRecordUpdate,
		Version:        "1.0.0",
		Category:       "record",
		Icon:           "pencil",
		RuntimeHandler: AutomationStepRecordUpdate,
		Active:         true,
		Schema:         capabilitySchemaFromStep(AutomationCapabilityRecordUpdate, "Update record", recordWriteStepSchema(AutomationStepRecordUpdate, "Update record")),
	},
	AutomationCapabilityRecordDelete: {
		Key:            AutomationCapabilityRecordDelete,
		Version:        "1.0.0",
		Category:       "record",
		Icon:           "trash",
		RuntimeHandler: AutomationStepRecordDelete,
		Active:         true,
		Schema:         capabilitySchemaFromStep(AutomationCapabilityRecordDelete, "Delete record", recordDeleteStepSchema()),
	},
}

func BuiltInAutomationCapabilities() map[string]AutomationCapabilityDefinition {
	result := make(map[string]AutomationCapabilityDefinition, len(builtInAutomationCapabilities))
	for key, capability := range builtInAutomationCapabilities {
		result[key] = capability
	}

	return result
}

func capabilitySchemaFromStep(key string, label string, stepSchema AutomationSchema) AutomationSchema {
	return AutomationSchema{
		Key:          key,
		Label:        label,
		Category:     stepSchema.Category,
		InputSchema:  stepSchema.InputSchema,
		OutputSchema: stepSchema.OutputSchema,
	}
}

func automationCapabilityKey(step map[string]any) string {
	key := strings.TrimSpace(toString(step["capability"]))
	if key == "" {
		key = strings.TrimSpace(toString(step["key"]))
	}
	return key
}

func automationCapabilityInput(step map[string]any) (map[string]any, error) {
	raw, ok := step["input"]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}

	input, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("capability step input must be a JSON object")
	}

	return input, nil
}

func resolveAutomationCapabilityHandler(app App, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("capability step requires a capability key")
	}

	if capability, ok := builtInAutomationCapabilities[key]; ok && capability.Active {
		return capability.RuntimeHandler, nil
	}

	capability, err := app.FindCapabilityByKey(key)
	if err != nil {
		return "", fmt.Errorf("missing or inactive capability %q", key)
	}

	handler := strings.TrimSpace(capability.RuntimeHandler())
	if _, ok := automationCapabilityHandlerStepTypes[handler]; !ok {
		return "", fmt.Errorf("capability %q uses unsupported runtime handler %q", key, handler)
	}

	return handler, nil
}

var automationCapabilityHandlerStepTypes = map[string]struct{}{
	AutomationStepHTTP:         {},
	AutomationStepMailSend:     {},
	AutomationStepRecordCreate: {},
	AutomationStepRecordUpdate: {},
	AutomationStepRecordDelete: {},
}

func automationCapabilityLegacyStep(app App, step map[string]any) (map[string]any, error) {
	key := automationCapabilityKey(step)
	handler, err := resolveAutomationCapabilityHandler(app, key)
	if err != nil {
		return nil, err
	}

	input, err := automationCapabilityInput(step)
	if err != nil {
		return nil, err
	}

	legacy := make(map[string]any, len(input)+1)
	for field, value := range input {
		if field == "type" {
			continue
		}
		legacy[field] = value
	}
	legacy["type"] = handler

	return legacy, nil
}
