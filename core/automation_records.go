package core

import (
	"fmt"
	"strings"
)

func executeAutomationRecordCreateStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	collection, err := automationStepCollection(ctx, step)
	if err != nil {
		return nil, err
	}

	record := NewRecord(collection)
	data, err := renderAutomationStepData(ctx, step["data"])
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		record.Set(key, value)
	}

	if err := ctx.App.Save(record); err != nil {
		return nil, err
	}

	return automationTemplateRecordData(record), nil
}

func executeAutomationRecordUpdateStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	record, err := automationStepTargetRecord(ctx, step)
	if err != nil {
		return nil, err
	}

	data, err := renderAutomationStepData(ctx, step["data"])
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		record.Set(key, value)
	}

	if err := ctx.App.Save(record); err != nil {
		return nil, err
	}

	return automationTemplateRecordData(record), nil
}

func executeAutomationRecordDeleteStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	record, err := automationStepTargetRecord(ctx, step)
	if err != nil {
		return nil, err
	}

	output := automationTemplateRecordData(record)
	output["deleted"] = true

	if err := ctx.App.Delete(record); err != nil {
		return nil, err
	}

	return output, nil
}

func automationStepCollection(ctx *automationExecutionContext, step map[string]any) (*Collection, error) {
	rendered, err := renderAutomationTemplateString(toString(step["collection"]), ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	collectionRef, ok := rendered.(string)
	if !ok || strings.TrimSpace(collectionRef) == "" {
		return nil, fmt.Errorf("record step is missing a valid collection")
	}

	collection, err := ctx.App.FindCollectionByNameOrId(collectionRef)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func renderAutomationStepData(ctx *automationExecutionContext, raw any) (map[string]any, error) {
	data, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("record step data must be a JSON object")
	}

	rendered, err := renderAutomationTemplateValue(data, ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	result, ok := rendered.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("record step data must be a JSON object")
	}

	return result, nil
}

func automationStepTargetRecord(ctx *automationExecutionContext, step map[string]any) (*Record, error) {
	collection, err := automationStepCollection(ctx, step)
	if err != nil {
		return nil, err
	}

	if rawID, ok := step["id"]; ok {
		renderedID, err := renderAutomationTemplateValue(rawID, ctx.TemplateData)
		if err != nil {
			return nil, err
		}

		recordID := strings.TrimSpace(fmt.Sprint(renderedID))
		if recordID != "" {
			return ctx.App.FindRecordById(collection, recordID)
		}
	}

	if rawFilter, ok := step["filter"]; ok {
		renderedFilter, err := renderAutomationTemplateValue(rawFilter, ctx.TemplateData)
		if err != nil {
			return nil, err
		}

		filter := strings.TrimSpace(fmt.Sprint(renderedFilter))
		if filter != "" {
			return ctx.App.FindFirstRecordByFilter(collection, filter)
		}
	}

	return nil, fmt.Errorf("record step requires either id or filter")
}
