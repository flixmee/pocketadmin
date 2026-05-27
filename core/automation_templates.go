package core

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
)

var (
	automationTemplatePattern      = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)
	automationWholeTemplatePattern = regexp.MustCompile(`^\s*\{\{\s*(.*?)\s*\}\}\s*$`)
	automationTemplateRootPattern  = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z0-9_$]*`)
	automationTemplatePathPattern  = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z0-9_$]*(\.[a-zA-Z_$][a-zA-Z0-9_$]*|\.[0-9]+)*$`)
)

const automationTemplateExpressionTimeout = 500 * time.Millisecond

func renderAutomationTemplateValue(value any, ctx map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		return renderAutomationTemplateString(v, ctx)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			rendered, err := renderAutomationTemplateValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			rendered, err := renderAutomationTemplateValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
		return result, nil
	default:
		return value, nil
	}
}

func renderAutomationTemplateString(value string, ctx map[string]any) (any, error) {
	if matches := automationWholeTemplatePattern.FindStringSubmatch(value); len(matches) == 2 {
		resolved, err := evalAutomationTemplateExpression(matches[1], ctx)
		if err != nil {
			return nil, err
		}

		if text, ok := resolved.(string); ok && len(text) > AutomationMaxTemplateStringSize {
			return nil, fmt.Errorf("automation template output exceeds %d bytes", AutomationMaxTemplateStringSize)
		}

		return resolved, nil
	}

	var renderErr error
	result := automationTemplatePattern.ReplaceAllStringFunc(value, func(match string) string {
		if renderErr != nil {
			return ""
		}

		matches := automationTemplatePattern.FindStringSubmatch(match)
		if len(matches) != 2 {
			return match
		}

		resolved, err := evalAutomationTemplateExpression(matches[1], ctx)
		if err != nil {
			renderErr = err
			return ""
		}

		if resolved == nil {
			return ""
		}

		return fmt.Sprint(resolved)
	})
	if renderErr != nil {
		return nil, renderErr
	}
	if len(result) > AutomationMaxTemplateStringSize {
		return nil, fmt.Errorf("automation template output exceeds %d bytes", AutomationMaxTemplateStringSize)
	}

	return result, nil
}

func evalAutomationTemplateExpression(expression string, ctx map[string]any) (any, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("empty automation template expression")
	}

	if automationTemplatePathPattern.MatchString(expression) {
		if resolved, ok := resolveAutomationTemplatePath(ctx, expression); ok {
			return resolved, nil
		}
	}

	vm := goja.New()
	for key, value := range automationTemplateJSContext(ctx) {
		if strings.HasPrefix(key, "__") {
			continue
		}
		if err := vm.Set(key, value); err != nil {
			return nil, fmt.Errorf("failed to initialize automation template root %q: %w", key, err)
		}
	}

	timeout := time.AfterFunc(automationTemplateExpressionTimeout, func() {
		vm.Interrupt("automation template expression timed out")
	})
	defer timeout.Stop()

	result, err := vm.RunString(expression)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate automation template expression %q: %w", expression, err)
	}
	if goja.IsUndefined(result) {
		return nil, fmt.Errorf("automation template expression %q resolved to undefined", expression)
	}

	return result.Export(), nil
}

func automationTemplateJSContext(ctx map[string]any) map[string]any {
	result := make(map[string]any, len(ctx))
	for key, value := range ctx {
		result[key] = value
	}

	app, _ := ctx[automationTemplateAppKey].(App)
	if app == nil {
		return result
	}

	if record := automationTemplateContextRecord(app, ctx, "record", automationTemplateRecordModelKey); record != nil {
		result["record"] = automationTemplateRecordDataWithRelations(app, record)
		result["trigger"] = automationTemplateTriggerDataWithRecord(result["trigger"], "$record", record)
	}
	if record := automationTemplateContextRecord(app, ctx, "recordOriginal", automationTemplateOriginalRecordKey); record != nil {
		result["recordOriginal"] = automationTemplateRecordDataWithRelations(app, record)
		result["trigger"] = automationTemplateTriggerDataWithRecord(result["trigger"], "$recordOriginal", record)
	}

	return result
}

func automationTemplateTriggerDataWithRecord(trigger any, key string, record *Record) map[string]any {
	result := map[string]any{}
	if triggerData, _ := trigger.(map[string]any); triggerData != nil {
		for k, v := range triggerData {
			result[k] = v
		}
	}
	if record != nil {
		result[key] = record
	}

	return result
}

func automationTemplateContextRecord(app App, ctx map[string]any, rootName string, modelKey string) *Record {
	if record, _ := ctx[modelKey].(*Record); record != nil {
		return record
	}

	root, ok := ctx[rootName]
	if !ok {
		return nil
	}

	return resolveAutomationTemplateRecordModel(app, ctx, rootName, root)
}

func automationTemplateRecordDataWithRelations(app App, record *Record) map[string]any {
	data := automationTemplateRecordData(record)
	if app == nil || record == nil || record.Collection() == nil {
		return data
	}

	for _, field := range record.Collection().Fields {
		relField, ok := field.(*RelationField)
		if !ok {
			continue
		}

		relValue, _, found := resolveAutomationRelationTemplateValue(app, record, relField)
		if found {
			data[relField.GetName()] = relValue
		} else if relField.IsMultiple() {
			data[relField.GetName()] = []any{}
		} else {
			data[relField.GetName()] = nil
		}
	}

	return data
}

func resolveAutomationTemplatePath(ctx map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		switch parts[0] {
		case "record":
			return resolveAutomationRecordTemplatePath(ctx, automationTemplateRecordModelKey, parts[1:])
		case "recordOriginal":
			return resolveAutomationRecordTemplatePath(ctx, automationTemplateOriginalRecordKey, parts[1:])
		}
	}

	return resolveAutomationGenericTemplatePath(ctx, parts)
}

func resolveAutomationGenericTemplatePath(root any, parts []string) (any, bool) {
	var current any = root
	for _, part := range parts {
		switch value := current.(type) {
		case map[string]any:
			next, ok := value[part]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}

	return current, true
}

func resolveAutomationRecordTemplatePath(ctx map[string]any, modelKey string, parts []string) (any, bool) {
	var rootName string
	switch modelKey {
	case automationTemplateRecordModelKey:
		rootName = "record"
	case automationTemplateOriginalRecordKey:
		rootName = "recordOriginal"
	default:
		return nil, false
	}

	root, ok := ctx[rootName]
	if !ok {
		return nil, false
	}

	record, _ := ctx[modelKey].(*Record)
	app, _ := ctx[automationTemplateAppKey].(App)
	if record == nil && app != nil {
		record = resolveAutomationTemplateRecordModel(app, ctx, rootName, root)
	}

	var current any = root
	currentRecord := record

	for i, part := range parts {
		if currentRecord != nil && app != nil && i < len(parts)-1 {
			if relField, ok := currentRecord.Collection().Fields.GetByName(part).(*RelationField); ok {
				relValue, relRecord, found := resolveAutomationRelationTemplateValue(app, currentRecord, relField)
				if !found {
					return nil, false
				}
				current = relValue
				currentRecord = relRecord
				continue
			}
		}

		switch value := current.(type) {
		case map[string]any:
			next, ok := value[part]
			if !ok {
				return nil, false
			}
			current = next
			currentRecord = nil
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
			currentRecord = nil
		default:
			return nil, false
		}
	}

	return current, true
}

func resolveAutomationTemplateRecordModel(app App, ctx map[string]any, rootName string, root any) *Record {
	rootData, ok := root.(map[string]any)
	if !ok {
		return nil
	}

	id := toString(rootData["id"])
	if id == "" {
		return nil
	}

	collectionId := ""
	if trigger, _ := ctx["trigger"].(map[string]any); trigger != nil {
		collectionId = toString(trigger["collectionId"])
	}
	if collectionId == "" {
		if collectionName := toString(rootData["collectionName"]); collectionName != "" {
			collectionId = collectionName
		}
	}
	if collectionId == "" {
		collectionId = toString(rootData["collectionId"])
	}
	if collectionId == "" && rootName == "recordOriginal" {
		if recordData, _ := ctx["record"].(map[string]any); recordData != nil {
			collectionId = toString(recordData["collectionId"])
		}
	}
	if collectionId == "" {
		return nil
	}

	record, err := app.FindRecordById(collectionId, id)
	if err != nil {
		return nil
	}

	return record
}

func resolveAutomationRelationTemplateValue(app App, record *Record, field *RelationField) (any, *Record, bool) {
	if field.IsMultiple() {
		ids := record.GetStringSlice(field.GetName())
		if len(ids) == 0 {
			return []any{}, nil, true
		}

		records, err := app.FindRecordsByIds(field.CollectionId, ids)
		if err != nil {
			return nil, nil, false
		}

		byId := make(map[string]*Record, len(records))
		for _, relRecord := range records {
			byId[relRecord.Id] = relRecord
		}

		result := make([]any, 0, len(ids))
		for _, id := range ids {
			if relRecord := byId[id]; relRecord != nil {
				result = append(result, automationTemplateRecordData(relRecord))
			}
		}

		return result, nil, true
	}

	id := record.GetString(field.GetName())
	if id == "" {
		return nil, nil, false
	}

	records, err := app.FindRecordsByIds(field.CollectionId, []string{id})
	if err != nil || len(records) == 0 {
		return nil, nil, false
	}

	return automationTemplateRecordData(records[0]), records[0], true
}

func validateAutomationTemplateRoots(value any) error {
	placeholders := collectAutomationTemplatePlaceholders(value)
	for _, placeholder := range placeholders {
		root := automationTemplateRootPattern.FindString(strings.TrimSpace(placeholder))

		switch root {
		case "trigger", "request", "i18n", "record", "recordOriginal", "automation", "run", "steps", "prevStep":
			continue
		case "", "Array", "Boolean", "Date", "JSON", "Math", "Number", "Object", "RegExp", "String", "parseFloat", "parseInt":
			continue
		default:
			return fmt.Errorf("unsupported automation template root %q", root)
		}
	}

	return nil
}

func collectAutomationTemplatePlaceholders(value any) []string {
	result := []string{}

	switch v := value.(type) {
	case string:
		matches := automationTemplatePattern.FindAllStringSubmatch(v, -1)
		for _, match := range matches {
			if len(match) == 2 {
				result = append(result, match[1])
			}
		}
	case []any:
		for _, item := range v {
			result = append(result, collectAutomationTemplatePlaceholders(item)...)
		}
	case map[string]any:
		for _, item := range v {
			result = append(result, collectAutomationTemplatePlaceholders(item)...)
		}
	}

	return result
}

func isNilAutomationValue(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func isEmptyAutomationValue(value any) bool {
	if isNilAutomationValue(value) {
		return true
	}

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	default:
		return false
	}
}

func automationValuesEqual(left any, right any) bool {
	return reflect.DeepEqual(left, right)
}

func automationValueIn(actual any, expected any) bool {
	values, ok := expected.([]any)
	if !ok {
		return automationValuesEqual(actual, expected)
	}

	for _, item := range values {
		if automationValuesEqual(actual, item) {
			return true
		}
	}

	return false
}

func automationValueStartsWith(actual any, expected any) bool {
	left, right, ok := automationComparableStrings(actual, expected)
	if !ok {
		return false
	}

	return strings.HasPrefix(left, right)
}

func automationValueEndsWith(actual any, expected any) bool {
	left, right, ok := automationComparableStrings(actual, expected)
	if !ok {
		return false
	}

	return strings.HasSuffix(left, right)
}

func automationValueContains(actual any, expected any) bool {
	left, right, ok := automationComparableStrings(actual, expected)
	if !ok {
		return false
	}

	return strings.Contains(left, right)
}

func automationComparableStrings(actual any, expected any) (string, string, bool) {
	if actual == nil || expected == nil {
		return "", "", false
	}

	return fmt.Sprint(actual), fmt.Sprint(expected), true
}
