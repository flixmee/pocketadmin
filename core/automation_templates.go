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

	return result, nil
}

func evalAutomationTemplateExpression(expression string, ctx map[string]any) (any, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("empty automation template expression")
	}

	vm := goja.New()
	for key, value := range ctx {
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

func resolveAutomationTemplatePath(ctx map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	var current any = ctx

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
