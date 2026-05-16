package core

import (
	"encoding/json"
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

type resolvedAutomationCapability struct {
	Handler        string
	ConnectorRef   string
	RequiredScopes []string
}

func resolveAutomationCapabilityHandler(app App, key string) (resolvedAutomationCapability, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return resolvedAutomationCapability{}, fmt.Errorf("capability step requires a capability key")
	}

	if capability, ok := builtInAutomationCapabilities[key]; ok && capability.Active {
		return resolvedAutomationCapability{Handler: capability.RuntimeHandler}, nil
	}

	capability, err := app.FindCapabilityByKey(key)
	if err != nil {
		return resolvedAutomationCapability{}, fmt.Errorf("missing or inactive capability %q", key)
	}

	handler := strings.TrimSpace(capability.RuntimeHandler())
	if _, ok := automationCapabilityHandlerStepTypes[handler]; !ok {
		return resolvedAutomationCapability{}, fmt.Errorf("capability %q uses unsupported runtime handler %q", key, handler)
	}

	return resolvedAutomationCapability{
		Handler:        handler,
		ConnectorRef:   capability.ConnectorRef(),
		RequiredScopes: jsonRawStringSlice(capability.RequiredScopes()),
	}, nil
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
	resolved, err := resolveAutomationCapabilityHandler(app, key)
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
	legacy["type"] = resolved.Handler

	connectorRef := strings.TrimSpace(toString(step["connectorRef"]))
	if connectorRef == "" {
		connectorRef = resolved.ConnectorRef
	}
	requiredScopes := append([]string{}, resolved.RequiredScopes...)
	requiredScopes = append(requiredScopes, stringSliceFromAny(step["requiredScopes"])...)
	if connectorRef != "" {
		if err := applyAutomationConnectorToLegacyStep(app, connectorRef, requiredScopes, legacy); err != nil {
			return nil, err
		}
	}

	return legacy, nil
}

func applyAutomationConnectorToLegacyStep(app App, connectorRef string, requiredScopes []string, legacy map[string]any) error {
	connector, err := app.FindConnectorById(connectorRef)
	if err != nil {
		return fmt.Errorf("missing connector %q", connectorRef)
	}
	if !connector.Active() {
		return fmt.Errorf("connector %q is inactive", connectorRef)
	}
	if err := ensureAutomationConnectorScopes(connector, requiredScopes); err != nil {
		return err
	}

	if strings.TrimSpace(toString(legacy["type"])) != AutomationStepHTTP {
		return nil
	}

	credentials := map[string]any{}
	if raw := strings.TrimSpace(connector.Credentials().String()); raw != "" {
		if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
			return err
		}
	}

	headers, _ := legacy["headers"].(map[string]any)
	if headers == nil {
		headers = map[string]any{}
	}
	switch connector.AuthType() {
	case AutomationConnectorAuthBearer:
		token := strings.TrimSpace(toString(credentials["token"]))
		if token == "" {
			token = strings.TrimSpace(toString(credentials["accessToken"]))
		}
		if token == "" {
			return fmt.Errorf("bearer connector %q is missing token", connectorRef)
		}
		headers["Authorization"] = "Bearer " + token
	case AutomationConnectorAuthAPIKey:
		key := strings.TrimSpace(toString(credentials["apiKey"]))
		if key == "" {
			key = strings.TrimSpace(toString(credentials["key"]))
		}
		if key == "" {
			return fmt.Errorf("api key connector %q is missing apiKey", connectorRef)
		}
		headerName := strings.TrimSpace(toString(credentials["headerName"]))
		if headerName == "" {
			headerName = "X-API-Key"
		}
		prefix := strings.TrimSpace(toString(credentials["prefix"]))
		if prefix != "" {
			key = prefix + " " + key
		}
		headers[headerName] = key
	}
	legacy["headers"] = headers

	return nil
}

func ensureAutomationConnectorScopes(connector *Connector, required []string) error {
	if len(required) == 0 {
		return nil
	}
	available := map[string]struct{}{}
	for _, scope := range jsonRawStringSlice(connector.Scopes()) {
		available[scope] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := available[scope]; !ok {
			return fmt.Errorf("connector %q is missing required scope %q", connector.Id, scope)
		}
	}
	return nil
}

func jsonRawStringSlice(raw fmt.Stringer) []string {
	if raw == nil || strings.TrimSpace(raw.String()) == "" {
		return nil
	}
	var values []any
	if err := json.Unmarshal([]byte(raw.String()), &values); err != nil {
		return nil
	}
	return stringSliceFromAny(values)
}

func stringSliceFromAny(value any) []string {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}
