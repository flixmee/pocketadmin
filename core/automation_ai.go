package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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

type settingsAutomationAIProvider struct {
	App App
}

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

	return settingsAutomationAIProvider{App: app}
}

func (p settingsAutomationAIProvider) RunAutomationAI(req AutomationAIRequest) (AutomationAIResponse, error) {
	if p.App == nil || p.App.Settings() == nil {
		return AutomationAIResponse{}, fmt.Errorf("AI settings are not available")
	}

	config := p.App.Settings().AI
	if !config.Enabled {
		return AutomationAIResponse{}, fmt.Errorf("AI settings are not enabled")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return AutomationAIResponse{}, fmt.Errorf("AI settings API key is required")
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = strings.TrimSpace(config.Model)
	}
	if model == "" {
		return AutomationAIResponse{}, fmt.Errorf("AI model is required")
	}

	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultAIProviderBaseURL(config.Provider)
	}
	if baseURL == "" {
		return AutomationAIResponse{}, fmt.Errorf("AI base URL is required")
	}

	prompt := automationAIPrompt(req)
	responseText, tokenUsage, err := callAutomationAIProvider(config.Provider, baseURL, config.APIKey, model, prompt)
	if err != nil {
		return AutomationAIResponse{}, err
	}

	output, err := automationAIResponseOutput(req, responseText)
	if err != nil {
		return AutomationAIResponse{}, err
	}

	return AutomationAIResponse{
		Output:     output,
		Model:      model,
		TokenUsage: tokenUsage,
	}, nil
}

func defaultAIProviderBaseURL(provider string) string {
	switch provider {
	case AIProviderOpenAI:
		return AIProviderOpenAIBaseURL
	case AIProviderGemini:
		return AIProviderGeminiBaseURL
	case AIProviderAnthropic:
		return AIProviderAnthropicBaseURL
	default:
		return AIProviderCustomBaseURL
	}
}

func automationAIPrompt(req AutomationAIRequest) string {
	input := fmt.Sprint(req.Input)

	switch req.StepType {
	case AutomationStepAIExtract:
		if len(req.Schema) > 0 {
			schema, _ := json.Marshal(req.Schema)
			return "Extract structured data from the input. Return JSON only matching this schema:\n" + string(schema) + "\n\nInput:\n" + input
		}
		return "Extract structured data from the input. Return JSON only.\n\nInput:\n" + input
	case AutomationStepAIClassify:
		return "Classify the input into exactly one of these labels: " + strings.Join(req.Labels, ", ") + ". Return JSON only with keys label and confidence.\n\nInput:\n" + input
	case AutomationStepAIGenerate:
		if len(req.Schema) > 0 {
			schema, _ := json.Marshal(req.Schema)
			return "Generate a response for the input. Return JSON only matching this schema:\n" + string(schema) + "\n\nInput:\n" + input
		}
		return input
	case AutomationStepAISummarize:
		return "Summarize the following input concisely:\n\n" + input
	default:
		return input
	}
}

func automationAIResponseOutput(req AutomationAIRequest, text string) (any, error) {
	text = strings.TrimSpace(text)

	switch req.StepType {
	case AutomationStepAIExtract:
		return parseAutomationAIJSONObject(text)
	case AutomationStepAIClassify:
		output, err := parseAutomationAIJSONObject(text)
		if err != nil {
			return nil, err
		}
		if output["label"] == nil && len(req.Labels) > 0 {
			output["label"] = req.Labels[0]
		}
		return output, nil
	case AutomationStepAIGenerate:
		if len(req.Schema) > 0 {
			return parseAutomationAIJSONObject(text)
		}
		return map[string]any{"text": text}, nil
	case AutomationStepAISummarize:
		return map[string]any{"summary": text}, nil
	default:
		return nil, fmt.Errorf("unsupported AI step type %q", req.StepType)
	}
}

func parseAutomationAIJSONObject(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	result := map[string]any{}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("AI response must be a JSON object: %w", err)
	}

	return result, nil
}

func callAutomationAIProvider(provider, baseURL, apiKey, model, prompt string) (string, map[string]int, error) {
	switch provider {
	case AIProviderGemini:
		return callAutomationAIGemini(baseURL, apiKey, model, prompt)
	case AIProviderAnthropic:
		return callAutomationAIAnthropic(baseURL, apiKey, model, prompt)
	default:
		return callAutomationAIOpenAICompatible(baseURL, apiKey, model, prompt)
	}
}

func callAutomationAIOpenAICompatible(baseURL, apiKey, model, prompt string) (string, map[string]int, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := automationAIJSONRequest(http.MethodPost, endpoint, apiKey, "", body, &response); err != nil {
		return "", nil, err
	}
	if len(response.Choices) == 0 {
		return "", nil, fmt.Errorf("AI provider returned no choices")
	}

	return response.Choices[0].Message.Content, map[string]int{
		"input":  response.Usage.PromptTokens,
		"output": response.Usage.CompletionTokens,
		"total":  response.Usage.TotalTokens,
	}, nil
}

func callAutomationAIGemini(baseURL, apiKey, model, prompt string) (string, map[string]int, error) {
	normalizedModel := strings.TrimPrefix(strings.TrimSpace(model), "models/")
	endpoint := strings.TrimRight(baseURL, "/") + "/models/" + url.PathEscape(normalizedModel) + ":generateContent"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", nil, err
	}
	q := parsed.Query()
	q.Set("key", apiKey)
	parsed.RawQuery = q.Encode()

	body := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := automationAIJSONRequest(http.MethodPost, parsed.String(), "", "", body, &response); err != nil {
		return "", nil, err
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return "", nil, fmt.Errorf("AI provider returned no candidates")
	}

	return response.Candidates[0].Content.Parts[0].Text, map[string]int{
		"input":  response.UsageMetadata.PromptTokenCount,
		"output": response.UsageMetadata.CandidatesTokenCount,
		"total":  response.UsageMetadata.TotalTokenCount,
	}, nil
}

func callAutomationAIAnthropic(baseURL, apiKey, model, prompt string) (string, map[string]int, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/messages"
	body := map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := automationAIJSONRequest(http.MethodPost, endpoint, "", apiKey, body, &response); err != nil {
		return "", nil, err
	}

	var parts []string
	for _, content := range response.Content {
		if content.Type == "" || content.Type == "text" {
			parts = append(parts, content.Text)
		}
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("AI provider returned no text content")
	}

	return strings.Join(parts, "\n"), map[string]int{
		"input":  response.Usage.InputTokens,
		"output": response.Usage.OutputTokens,
		"total":  response.Usage.InputTokens + response.Usage.OutputTokens,
	}, nil
}

func automationAIJSONRequest(method, endpoint, bearerKey, anthropicKey string, body any, result any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(method, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	if bearerKey != "" {
		req.Header.Set("authorization", "Bearer "+bearerKey)
	}
	if anthropicKey != "" {
		req.Header.Set("x-api-key", anthropicKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("AI provider returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}

	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("failed to decode AI provider response: %w", err)
	}

	return nil
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
