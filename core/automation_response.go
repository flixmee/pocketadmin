package core

import (
	"fmt"
	"net/http"
)

func executeAutomationResponseStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	statusCode, err := automationResponseStatusCode(step["statusCode"], http.StatusOK)
	if err != nil {
		return nil, err
	}

	headers, err := renderAutomationResponseHeaders(ctx, step["headers"])
	if err != nil {
		return nil, err
	}

	var body any
	if rawBody, ok := step["body"]; ok {
		body, err = renderAutomationTemplateValue(rawBody, ctx.TemplateData)
		if err != nil {
			return nil, err
		}
	}

	response := &AutomationWebhookResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
	ctx.WebhookResponse = response

	output := map[string]any{
		"statusCode": response.StatusCode,
	}
	if len(response.Headers) > 0 {
		output["headers"] = stringMapToAnyMap(response.Headers)
	}
	if response.Body != nil {
		output["body"] = response.Body
	}

	return output, nil
}

func renderAutomationResponseHeaders(ctx *automationExecutionContext, raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}

	headers, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response step headers must be a JSON object")
	}

	rendered, err := renderAutomationTemplateValue(headers, ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	renderedHeaders, ok := rendered.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response step headers must be a JSON object")
	}

	result := make(map[string]string, len(renderedHeaders))
	for key, value := range renderedHeaders {
		result[key] = fmt.Sprint(value)
	}

	return result, nil
}

func automationResponseStatusCode(value any, fallback int) (int, error) {
	if value == nil {
		return fallback, nil
	}

	var statusCode int
	switch v := value.(type) {
	case int:
		statusCode = v
	case int64:
		statusCode = int(v)
	case float64:
		statusCode = int(v)
		if float64(statusCode) != v {
			return 0, fmt.Errorf("response step statusCode must be an integer")
		}
	case float32:
		statusCode = int(v)
		if float32(statusCode) != v {
			return 0, fmt.Errorf("response step statusCode must be an integer")
		}
	default:
		return 0, fmt.Errorf("response step statusCode must be an integer")
	}

	if statusCode < 100 || statusCode > 599 {
		return 0, fmt.Errorf("response step statusCode must be between 100 and 599")
	}

	return statusCode, nil
}
