package core

// AutomationSchemaCatalog describes the typed contracts currently known by the
// automation runtime. It intentionally uses JSON Schema-compatible maps so the
// API can expose the data without binding the admin UI to Go-only types.
type AutomationSchemaCatalog struct {
	Triggers     map[string]AutomationSchema               `json:"triggers"`
	Steps        map[string]AutomationSchema               `json:"steps"`
	Capabilities map[string]AutomationCapabilityDefinition `json:"capabilities"`
	Limits       AutomationRuntimeLimits                   `json:"limits"`
}

type AutomationSchema struct {
	Key          string         `json:"key"`
	Label        string         `json:"label"`
	Category     string         `json:"category"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

type AutomationRuntimeLimits struct {
	MaxSteps              int `json:"maxSteps"`
	MaxTemplateStringSize int `json:"maxTemplateStringSize"`
	HTTPMaxTimeoutSeconds int `json:"httpMaxTimeoutSeconds"`
	HTTPInputBodyLimit    int `json:"httpInputBodyLimit"`
	HTTPOutputBodyLimit   int `json:"httpOutputBodyLimit"`
}

// AutomationSchemas returns the built-in automation trigger and step schemas.
func AutomationSchemas() AutomationSchemaCatalog {
	return AutomationSchemaCatalog{
		Triggers: map[string]AutomationSchema{
			AutomationTriggerRecordCreate:   recordTriggerSchema(AutomationTriggerRecordCreate, "Record created"),
			AutomationTriggerRecordUpdate:   recordTriggerSchema(AutomationTriggerRecordUpdate, "Record updated"),
			AutomationTriggerRecordDelete:   recordTriggerSchema(AutomationTriggerRecordDelete, "Record deleted"),
			AutomationTriggerScheduleCron:   simpleTriggerSchema(AutomationTriggerScheduleCron, "Schedule"),
			AutomationTriggerWebhook:        webhookTriggerSchema(),
			AutomationTriggerManual:         simpleTriggerSchema(AutomationTriggerManual, "Manual"),
			AutomationTriggerI18nMissing:    i18nTriggerSchema(AutomationTriggerI18nMissing, "Translation missing"),
			AutomationTriggerI18nPublished:  i18nTriggerSchema(AutomationTriggerI18nPublished, "Locale published"),
			AutomationTriggerI18nUpdated:    i18nTriggerSchema(AutomationTriggerI18nUpdated, "Translation updated"),
			AutomationTriggerI18nAIFinished: i18nTriggerSchema(AutomationTriggerI18nAIFinished, "AI translation finished"),
		},
		Steps: map[string]AutomationSchema{
			AutomationStepCondition:    conditionStepSchema(),
			AutomationStepHTTP:         httpStepSchema(),
			AutomationStepMailSend:     mailStepSchema(),
			AutomationStepRecordCreate: recordWriteStepSchema(AutomationStepRecordCreate, "Create record"),
			AutomationStepRecordUpdate: recordWriteStepSchema(AutomationStepRecordUpdate, "Update record"),
			AutomationStepRecordDelete: recordDeleteStepSchema(),
			AutomationStepResponse:     responseStepSchema(),
			AutomationStepCapability:   capabilityStepSchema(),
			AutomationStepWaitDelay:    waitDelayStepSchema(),
			AutomationStepWaitWebhook:  waitKeyStepSchema(AutomationStepWaitWebhook, "Wait for webhook"),
			AutomationStepWaitEvent:    waitKeyStepSchema(AutomationStepWaitEvent, "Wait for event"),
			AutomationStepWaitApproval: waitApprovalStepSchema(),
			AutomationStepAIExtract:    aiStepSchema(AutomationStepAIExtract, "AI extract"),
			AutomationStepAIClassify:   aiStepSchema(AutomationStepAIClassify, "AI classify"),
			AutomationStepAIGenerate:   aiStepSchema(AutomationStepAIGenerate, "AI generate"),
			AutomationStepAISummarize:  aiStepSchema(AutomationStepAISummarize, "AI summarize"),
		},
		Capabilities: BuiltInAutomationCapabilities(),
		Limits: AutomationRuntimeLimits{
			MaxSteps:              AutomationMaxSteps,
			MaxTemplateStringSize: AutomationMaxTemplateStringSize,
			HTTPMaxTimeoutSeconds: int(AutomationHTTPMaxTimeout.Seconds()),
			HTTPInputBodyLimit:    AutomationHTTPInputBodyLimit,
			HTTPOutputBodyLimit:   automationHTTPOutputBodyLimit,
		},
	}
}

func aiStepSchema(key string, label string) AutomationSchema {
	input := map[string]any{
		"type":   constStringSchema(key),
		"model":  stringSchema(),
		"input":  map[string]any{},
		"schema": objectSchema(nil),
	}
	if key == AutomationStepAIClassify {
		input["labels"] = arraySchema(stringSchema())
	}

	return AutomationSchema{
		Key:         key,
		Label:       label,
		Category:    "ai",
		InputSchema: objectSchema(input),
		OutputSchema: objectSchema(map[string]any{
			"model":      stringSchema(),
			"output":     map[string]any{},
			"tokenUsage": objectSchema(nil),
		}),
	}
}

func waitDelayStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepWaitDelay,
		Label:    "Wait delay",
		Category: "wait",
		InputSchema: objectSchema(map[string]any{
			"type":     constStringSchema(AutomationStepWaitDelay),
			"duration": stringSchema(),
		}),
		OutputSchema: waitOutputSchema(),
	}
}

func waitKeyStepSchema(key string, label string) AutomationSchema {
	return AutomationSchema{
		Key:      key,
		Label:    label,
		Category: "wait",
		InputSchema: objectSchema(map[string]any{
			"type": constStringSchema(key),
			"key":  stringSchema(),
		}),
		OutputSchema: waitOutputSchema(),
	}
}

func waitApprovalStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepWaitApproval,
		Label:    "Wait for approval",
		Category: "approval",
		InputSchema: objectSchema(map[string]any{
			"type":     constStringSchema(AutomationStepWaitApproval),
			"assignee": stringSchema(),
			"role":     stringSchema(),
		}),
		OutputSchema: objectSchema(map[string]any{
			"waiting":    boolSchema(),
			"type":       stringSchema(),
			"token":      stringSchema(),
			"approvalId": stringSchema(),
			"role":       stringSchema(),
			"assignee":   stringSchema(),
		}),
	}
}

func waitOutputSchema() map[string]any {
	return objectSchema(map[string]any{
		"waiting":  boolSchema(),
		"type":     stringSchema(),
		"token":    stringSchema(),
		"key":      stringSchema(),
		"resumeAt": stringSchema(),
	})
}

func capabilityStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepCapability,
		Label:    "Capability",
		Category: "capability",
		InputSchema: objectSchema(map[string]any{
			"type":       constStringSchema(AutomationStepCapability),
			"capability": stringSchema(),
			"key":        stringSchema(),
			"input":      objectSchema(nil),
		}),
		OutputSchema: objectSchema(nil),
	}
}

func simpleTriggerSchema(key string, label string) AutomationSchema {
	return AutomationSchema{
		Key:      key,
		Label:    label,
		Category: "trigger",
		OutputSchema: objectSchema(map[string]any{
			"trigger": objectSchema(map[string]any{
				"type": stringSchema(),
			}),
		}),
	}
}

func recordTriggerSchema(key string, label string) AutomationSchema {
	schema := simpleTriggerSchema(key, label)
	schema.OutputSchema = objectSchema(map[string]any{
		"trigger": objectSchema(map[string]any{
			"type":           stringSchema(),
			"collectionId":   stringSchema(),
			"collectionName": stringSchema(),
		}),
		"record":         objectSchema(nil),
		"recordOriginal": objectSchema(nil),
	})
	return schema
}

func webhookTriggerSchema() AutomationSchema {
	schema := simpleTriggerSchema(AutomationTriggerWebhook, "Webhook")
	schema.OutputSchema = objectSchema(map[string]any{
		"trigger": objectSchema(map[string]any{
			"type":    stringSchema(),
			"request": objectSchema(nil),
		}),
		"request": objectSchema(map[string]any{
			"method":   stringSchema(),
			"path":     stringSchema(),
			"query":    objectSchema(nil),
			"headers":  objectSchema(nil),
			"body":     map[string]any{},
			"remoteIP": stringSchema(),
		}),
	})
	return schema
}

func i18nTriggerSchema(key string, label string) AutomationSchema {
	schema := simpleTriggerSchema(key, label)
	schema.OutputSchema = objectSchema(map[string]any{
		"trigger": objectSchema(map[string]any{
			"type":           stringSchema(),
			"collectionId":   stringSchema(),
			"collectionName": stringSchema(),
			"i18n":           objectSchema(nil),
		}),
		"i18n":   objectSchema(nil),
		"record": objectSchema(nil),
	})
	return schema
}

func conditionStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepCondition,
		Label:    "Condition",
		Category: "control",
		InputSchema: objectSchema(map[string]any{
			"type":  constStringSchema(AutomationStepCondition),
			"path":  stringSchema(),
			"field": stringSchema(),
			"op": enumSchema([]string{
				automationConditionOpEq,
				automationConditionOpNeq,
				automationConditionOpIn,
				automationConditionOpExists,
				automationConditionOpEmpty,
				automationConditionOpNotEmpty,
				automationConditionOpStartsWith,
				automationConditionOpEndsWith,
				automationConditionOpNotStartsWith,
				automationConditionOpNotEndsWith,
				automationConditionOpContains,
			}),
			"value": map[string]any{},
			"match": enumSchema([]string{"and", "or"}),
			"conditions": arraySchema(objectSchema(map[string]any{
				"path":  stringSchema(),
				"field": stringSchema(),
				"op": enumSchema([]string{
					automationConditionOpEq,
					automationConditionOpNeq,
					automationConditionOpIn,
					automationConditionOpExists,
					automationConditionOpEmpty,
					automationConditionOpNotEmpty,
					automationConditionOpStartsWith,
					automationConditionOpEndsWith,
					automationConditionOpNotStartsWith,
					automationConditionOpNotEndsWith,
					automationConditionOpContains,
				}),
				"value": map[string]any{},
			})),
		}),
		OutputSchema: objectSchema(map[string]any{
			"path":     stringSchema(),
			"op":       stringSchema(),
			"found":    boolSchema(),
			"matched":  boolSchema(),
			"actual":   map[string]any{},
			"expected": map[string]any{},
		}),
	}
}

func httpStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepHTTP,
		Label:    "HTTP request",
		Category: "integration",
		InputSchema: objectSchema(map[string]any{
			"type":    constStringSchema(AutomationStepHTTP),
			"method":  stringSchema(),
			"url":     stringSchema(),
			"headers": objectSchema(nil),
			"body":    map[string]any{},
			"timeout": numberSchema(),
		}),
		OutputSchema: objectSchema(map[string]any{
			"status":     stringSchema(),
			"statusCode": numberSchema(),
			"headers":    objectSchema(nil),
			"body":       map[string]any{},
			"bodyText":   stringSchema(),
		}),
	}
}

func mailStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepMailSend,
		Label:    "Send email",
		Category: "communication",
		InputSchema: objectSchema(map[string]any{
			"type":        constStringSchema(AutomationStepMailSend),
			"to":          arraySchema(stringSchema()),
			"cc":          arraySchema(stringSchema()),
			"bcc":         arraySchema(stringSchema()),
			"subject":     stringSchema(),
			"text":        stringSchema(),
			"html":        stringSchema(),
			"attachments": arraySchema(stringSchema()),
		}),
		OutputSchema: objectSchema(map[string]any{
			"sent": boolSchema(),
		}),
	}
}

func recordWriteStepSchema(key string, label string) AutomationSchema {
	return AutomationSchema{
		Key:      key,
		Label:    label,
		Category: "record",
		InputSchema: objectSchema(map[string]any{
			"type":       constStringSchema(key),
			"collection": stringSchema(),
			"id":         stringSchema(),
			"filter":     stringSchema(),
			"data":       objectSchema(nil),
		}),
		OutputSchema: objectSchema(map[string]any{
			"id":      stringSchema(),
			"created": stringSchema(),
			"updated": stringSchema(),
		}),
	}
}

func recordDeleteStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepRecordDelete,
		Label:    "Delete record",
		Category: "record",
		InputSchema: objectSchema(map[string]any{
			"type":       constStringSchema(AutomationStepRecordDelete),
			"collection": stringSchema(),
			"id":         stringSchema(),
			"filter":     stringSchema(),
		}),
		OutputSchema: objectSchema(map[string]any{
			"deleted": boolSchema(),
			"id":      stringSchema(),
		}),
	}
}

func responseStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepResponse,
		Label:    "Webhook response",
		Category: "webhook",
		InputSchema: objectSchema(map[string]any{
			"type":       constStringSchema(AutomationStepResponse),
			"statusCode": numberSchema(),
			"headers":    objectSchema(nil),
			"body":       map[string]any{},
		}),
		OutputSchema: objectSchema(map[string]any{
			"statusCode": numberSchema(),
			"headers":    objectSchema(nil),
			"body":       map[string]any{},
		}),
	}
}

func objectSchema(properties map[string]any) map[string]any {
	schema := map[string]any{"type": "object"}
	if properties != nil {
		schema["properties"] = properties
	}
	return schema
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": items,
	}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func constStringSchema(value string) map[string]any {
	return map[string]any{
		"type":  "string",
		"const": value,
	}
}

func numberSchema() map[string]any {
	return map[string]any{"type": "number"}
}

func boolSchema() map[string]any {
	return map[string]any{"type": "boolean"}
}

func enumSchema(values []string) map[string]any {
	items := make([]any, len(values))
	for i, value := range values {
		items[i] = value
	}

	return map[string]any{
		"type": "string",
		"enum": items,
	}
}
