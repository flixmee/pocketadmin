package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	StoreKeyKnowledgeProvider = "pbAppKnowledgeProvider"

	knowledgeEmbeddingRequestTimeout  = 30 * time.Second
	knowledgeGenerationRequestTimeout = 90 * time.Second
	knowledgeStreamingRequestTimeout  = 2 * time.Minute
)

var sharedKnowledgeHTTPClient = newKnowledgeHTTPClient()

// KnowledgeProvider supplies the two model operations needed by the knowledge
// base. Applications can store their own implementation under
// StoreKeyKnowledgeProvider to use a self-hosted model or another vendor.
type KnowledgeProvider interface {
	Embed(ctx context.Context, input []string) ([][]float32, error)
	Generate(ctx context.Context, prompt string) (KnowledgeGenerationResult, error)
}

// KnowledgeStreamingProvider is an optional provider extension that emits
// generation deltas as soon as the upstream model returns them.
type KnowledgeStreamingProvider interface {
	GenerateStream(
		ctx context.Context,
		prompt string,
		onDelta func(string) error,
	) (KnowledgeGenerationResult, error)
}

type KnowledgeGenerationResult struct {
	Text       string         `json:"text"`
	Model      string         `json:"model,omitempty"`
	TokenUsage map[string]int `json:"tokenUsage,omitempty"`
}

// SettingsKnowledgeProvider uses the app AI settings. OpenAI-compatible and
// Gemini endpoints support both embeddings and generation. Anthropic supports
// generation only, so deployments using it must install a custom provider for
// embeddings.
type SettingsKnowledgeProvider struct {
	App    App
	Client *http.Client
}

func ResolveKnowledgeProvider(app App) KnowledgeProvider {
	if provider, ok := app.Store().Get(StoreKeyKnowledgeProvider).(KnowledgeProvider); ok && provider != nil {
		return provider
	}

	return &SettingsKnowledgeProvider{App: app}
}

func (p *SettingsKnowledgeProvider) Embed(ctx context.Context, input []string) ([][]float32, error) {
	config, err := p.config()
	if err != nil {
		return nil, err
	}
	if len(input) == 0 {
		return [][]float32{}, nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, knowledgeEmbeddingRequestTimeout)
	defer cancel()

	model := strings.TrimSpace(config.EmbeddingModel)
	if model == "" {
		switch config.Provider {
		case AIProviderGemini:
			model = "gemini-embedding-2"
		case AIProviderAnthropic:
			return nil, errors.New("Anthropic does not provide embeddings; configure a custom knowledge provider")
		case AIProviderCustom:
			return nil, errors.New("AI embedding model is required for custom provider")
		default:
			model = "text-embedding-3-small"
		}
	}

	switch config.Provider {
	case AIProviderGemini:
		return p.embedGemini(requestCtx, config, model, input)
	case AIProviderAnthropic:
		return nil, errors.New("Anthropic does not provide embeddings; configure a custom knowledge provider")
	default:
		return p.embedOpenAICompatible(requestCtx, config, model, input)
	}
}

func (p *SettingsKnowledgeProvider) Generate(ctx context.Context, prompt string) (KnowledgeGenerationResult, error) {
	config, err := p.config()
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		return KnowledgeGenerationResult{}, errors.New("AI model is required")
	}
	requestCtx, cancel := context.WithTimeout(ctx, knowledgeGenerationRequestTimeout)
	defer cancel()

	switch config.Provider {
	case AIProviderGemini:
		return p.generateGemini(requestCtx, config, model, prompt)
	case AIProviderAnthropic:
		return p.generateAnthropic(requestCtx, config, model, prompt)
	default:
		return p.generateOpenAICompatible(requestCtx, config, model, prompt)
	}
}

func (p *SettingsKnowledgeProvider) GenerateStream(
	ctx context.Context,
	prompt string,
	onDelta func(string) error,
) (KnowledgeGenerationResult, error) {
	config, err := p.config()
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return KnowledgeGenerationResult{}, errors.New("AI model is required")
	}
	requestCtx, cancel := context.WithTimeout(ctx, knowledgeStreamingRequestTimeout)
	defer cancel()

	switch config.Provider {
	case AIProviderGemini:
		return p.generateGeminiStream(requestCtx, config, model, prompt, onDelta)
	case AIProviderAnthropic:
		return p.generateAnthropicStream(requestCtx, config, model, prompt, onDelta)
	default:
		return p.generateOpenAICompatibleStream(requestCtx, config, model, prompt, onDelta)
	}
}

func (p *SettingsKnowledgeProvider) KnowledgeCacheKey() string {
	config, err := p.config()
	if err != nil {
		return ""
	}
	return knowledgeCacheHash(
		config.Provider,
		strings.TrimRight(config.BaseURL, "/"),
		strings.TrimSpace(config.EmbeddingModel),
		config.APIKey,
	)
}

func (p *SettingsKnowledgeProvider) config() (AIConfig, error) {
	if p == nil || p.App == nil || p.App.Settings() == nil {
		return AIConfig{}, errors.New("AI settings are not available")
	}

	config := p.App.Settings().AI
	if !config.Enabled {
		return AIConfig{}, errors.New("AI settings are not enabled")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return AIConfig{}, errors.New("AI settings API key is required")
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = defaultAIProviderBaseURL(config.Provider)
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		return AIConfig{}, errors.New("AI base URL is required")
	}

	return config, nil
}

func (p *SettingsKnowledgeProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return sharedKnowledgeHTTPClient
}

func newKnowledgeHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.ExpectContinueTimeout = time.Second
	return &http.Client{Transport: transport}
}

func (p *SettingsKnowledgeProvider) embedOpenAICompatible(
	ctx context.Context,
	config AIConfig,
	model string,
	input []string,
) ([][]float32, error) {
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	err := p.jsonRequest(ctx, config, strings.TrimRight(config.BaseURL, "/")+"/embeddings", map[string]any{
		"model": model,
		"input": input,
	}, &response)
	if err != nil {
		return nil, err
	}
	if len(response.Data) != len(input) {
		return nil, fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(response.Data), len(input))
	}

	result := make([][]float32, len(response.Data))
	seen := make([]bool, len(response.Data))
	for _, item := range response.Data {
		if item.Index < 0 || item.Index >= len(result) || seen[item.Index] {
			return nil, fmt.Errorf("embedding provider returned invalid index %d", item.Index)
		}
		seen[item.Index] = true
		result[item.Index] = item.Embedding
	}

	return validateKnowledgeEmbeddings(result)
}

func (p *SettingsKnowledgeProvider) embedGemini(
	ctx context.Context,
	config AIConfig,
	model string,
	input []string,
) ([][]float32, error) {
	normalizedModel := strings.TrimPrefix(model, "models/")
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/models/" + url.PathEscape(normalizedModel) + ":batchEmbedContents"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("key", config.APIKey)
	parsed.RawQuery = query.Encode()

	requests := make([]map[string]any, len(input))
	for i, text := range input {
		requests[i] = map[string]any{
			"model": "models/" + normalizedModel,
			"content": map[string]any{
				"parts": []map[string]string{{"text": text}},
			},
		}
	}

	var response struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	if err := p.jsonRequest(ctx, config, parsed.String(), map[string]any{"requests": requests}, &response); err != nil {
		return nil, err
	}
	if len(response.Embeddings) != len(input) {
		return nil, fmt.Errorf("embedding provider returned %d vectors for %d inputs", len(response.Embeddings), len(input))
	}

	result := make([][]float32, len(response.Embeddings))
	for i, item := range response.Embeddings {
		result[i] = item.Values
	}

	return validateKnowledgeEmbeddings(result)
}

func (p *SettingsKnowledgeProvider) generateOpenAICompatible(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
) (KnowledgeGenerationResult, error) {
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

	err := p.jsonRequest(ctx, config, strings.TrimRight(config.BaseURL, "/")+"/chat/completions", map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}, &response)
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}
	if len(response.Choices) == 0 {
		return KnowledgeGenerationResult{}, errors.New("AI provider returned no choices")
	}

	return KnowledgeGenerationResult{
		Text:  strings.TrimSpace(response.Choices[0].Message.Content),
		Model: model,
		TokenUsage: map[string]int{
			"input":  response.Usage.PromptTokens,
			"output": response.Usage.CompletionTokens,
			"total":  response.Usage.TotalTokens,
		},
	}, nil
}

func (p *SettingsKnowledgeProvider) generateGemini(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
) (KnowledgeGenerationResult, error) {
	normalizedModel := strings.TrimPrefix(model, "models/")
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/models/" + url.PathEscape(normalizedModel) + ":generateContent"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}
	query := parsed.Query()
	query.Set("key", config.APIKey)
	parsed.RawQuery = query.Encode()

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

	err = p.jsonRequest(ctx, config, parsed.String(), map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]string{{"text": prompt}},
		}},
	}, &response)
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return KnowledgeGenerationResult{}, errors.New("AI provider returned no candidates")
	}

	parts := make([]string, 0, len(response.Candidates[0].Content.Parts))
	for _, part := range response.Candidates[0].Content.Parts {
		parts = append(parts, part.Text)
	}

	return KnowledgeGenerationResult{
		Text:  strings.TrimSpace(strings.Join(parts, "\n")),
		Model: model,
		TokenUsage: map[string]int{
			"input":  response.UsageMetadata.PromptTokenCount,
			"output": response.UsageMetadata.CandidatesTokenCount,
			"total":  response.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *SettingsKnowledgeProvider) generateAnthropic(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
) (KnowledgeGenerationResult, error) {
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

	err := p.jsonRequest(ctx, config, strings.TrimRight(config.BaseURL, "/")+"/messages", map[string]any{
		"model":      model,
		"max_tokens": 1200,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}, &response)
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}

	parts := make([]string, 0, len(response.Content))
	for _, part := range response.Content {
		if part.Type == "" || part.Type == "text" {
			parts = append(parts, part.Text)
		}
	}
	if len(parts) == 0 {
		return KnowledgeGenerationResult{}, errors.New("AI provider returned no text content")
	}

	return KnowledgeGenerationResult{
		Text:  strings.TrimSpace(strings.Join(parts, "\n")),
		Model: model,
		TokenUsage: map[string]int{
			"input":  response.Usage.InputTokens,
			"output": response.Usage.OutputTokens,
			"total":  response.Usage.InputTokens + response.Usage.OutputTokens,
		},
	}, nil
}

func (p *SettingsKnowledgeProvider) generateOpenAICompatibleStream(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
	onDelta func(string) error,
) (KnowledgeGenerationResult, error) {
	var text strings.Builder
	result := KnowledgeGenerationResult{Model: model}
	err := p.sseRequest(
		ctx,
		config,
		strings.TrimRight(config.BaseURL, "/")+"/chat/completions",
		map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"stream":         true,
			"stream_options": map[string]bool{"include_usage": true},
		},
		func(_ string, raw []byte) error {
			if string(raw) == "[DONE]" {
				return nil
			}
			var event struct {
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				return fmt.Errorf("invalid AI provider stream event: %w", err)
			}
			if event.Model != "" {
				result.Model = event.Model
			}
			result.TokenUsage = knowledgeTokenUsage(
				event.Usage.PromptTokens,
				event.Usage.CompletionTokens,
				event.Usage.TotalTokens,
				result.TokenUsage,
			)
			for _, choice := range event.Choices {
				if err := appendKnowledgeDelta(&text, choice.Delta.Content, onDelta); err != nil {
					return err
				}
			}
			return nil
		},
	)
	result.Text = strings.TrimSpace(text.String())
	return result, err
}

func (p *SettingsKnowledgeProvider) generateGeminiStream(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
	onDelta func(string) error,
) (KnowledgeGenerationResult, error) {
	normalizedModel := strings.TrimPrefix(model, "models/")
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/models/" + url.PathEscape(normalizedModel) +
		":streamGenerateContent"
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return KnowledgeGenerationResult{}, err
	}
	query := parsed.Query()
	query.Set("alt", "sse")
	query.Set("key", config.APIKey)
	parsed.RawQuery = query.Encode()

	var text strings.Builder
	result := KnowledgeGenerationResult{Model: model}
	err = p.sseRequest(
		ctx,
		config,
		parsed.String(),
		map[string]any{
			"contents": []map[string]any{{
				"role":  "user",
				"parts": []map[string]string{{"text": prompt}},
			}},
		},
		func(_ string, raw []byte) error {
			var event struct {
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
			if err := json.Unmarshal(raw, &event); err != nil {
				return fmt.Errorf("invalid Gemini stream event: %w", err)
			}
			result.TokenUsage = knowledgeTokenUsage(
				event.UsageMetadata.PromptTokenCount,
				event.UsageMetadata.CandidatesTokenCount,
				event.UsageMetadata.TotalTokenCount,
				result.TokenUsage,
			)
			for _, candidate := range event.Candidates {
				for _, part := range candidate.Content.Parts {
					if err := appendKnowledgeDelta(&text, part.Text, onDelta); err != nil {
						return err
					}
				}
			}
			return nil
		},
	)
	result.Text = strings.TrimSpace(text.String())
	return result, err
}

func (p *SettingsKnowledgeProvider) generateAnthropicStream(
	ctx context.Context,
	config AIConfig,
	model string,
	prompt string,
	onDelta func(string) error,
) (KnowledgeGenerationResult, error) {
	var text strings.Builder
	result := KnowledgeGenerationResult{Model: model}
	inputTokens := 0
	outputTokens := 0
	err := p.sseRequest(
		ctx,
		config,
		strings.TrimRight(config.BaseURL, "/")+"/messages",
		map[string]any{
			"model":      model,
			"max_tokens": 1200,
			"stream":     true,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		},
		func(_ string, raw []byte) error {
			var event struct {
				Message struct {
					Model string `json:"model"`
					Usage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				return fmt.Errorf("invalid Anthropic stream event: %w", err)
			}
			if event.Message.Model != "" {
				result.Model = event.Message.Model
			}
			if event.Message.Usage.InputTokens > 0 {
				inputTokens = event.Message.Usage.InputTokens
			}
			if event.Usage.OutputTokens > 0 {
				outputTokens = event.Usage.OutputTokens
			}
			result.TokenUsage = knowledgeTokenUsage(inputTokens, outputTokens, inputTokens+outputTokens, result.TokenUsage)
			return appendKnowledgeDelta(&text, event.Delta.Text, onDelta)
		},
	)
	result.Text = strings.TrimSpace(text.String())
	return result, err
}

func (p *SettingsKnowledgeProvider) sseRequest(
	ctx context.Context,
	config AIConfig,
	endpoint string,
	body any,
	onEvent func(string, []byte) error,
) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	if config.Provider == AIProviderAnthropic {
		req.Header.Set("x-api-key", config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if config.Provider != AIProviderGemini {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	res, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 8*1024*1024))
		if readErr != nil {
			return readErr
		}
		return fmt.Errorf("AI provider returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}

	return readKnowledgeSSE(res.Body, onEvent)
}

func readKnowledgeSSE(reader io.Reader, onEvent func(string, []byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 8*1024*1024)
	eventName := "message"
	dataLines := make([]string, 0, 1)
	dispatch := func() error {
		if len(dataLines) == 0 {
			eventName = "message"
			return nil
		}
		raw := []byte(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		name := eventName
		eventName = "message"
		if name == "error" {
			return fmt.Errorf("AI provider stream error: %s", strings.TrimSpace(string(raw)))
		}
		return onEvent(name, raw)
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case line == "":
			if err := dispatch(); err != nil {
				return err
			}
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func appendKnowledgeDelta(builder *strings.Builder, delta string, onDelta func(string) error) error {
	if delta == "" {
		return nil
	}
	builder.WriteString(delta)
	if onDelta != nil {
		return onDelta(delta)
	}
	return nil
}

func knowledgeTokenUsage(input, output, total int, fallback map[string]int) map[string]int {
	if input == 0 && output == 0 && total == 0 {
		return fallback
	}
	if total == 0 {
		total = input + output
	}
	return map[string]int{"input": input, "output": output, "total": total}
}

func (p *SettingsKnowledgeProvider) jsonRequest(
	ctx context.Context,
	config AIConfig,
	endpoint string,
	body any,
	result any,
) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if config.Provider == AIProviderAnthropic {
		req.Header.Set("x-api-key", config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if config.Provider != AIProviderGemini {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	res, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 8*1024*1024))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("AI provider returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("invalid AI provider response: %w", err)
	}

	return nil
}

func validateKnowledgeEmbeddings(vectors [][]float32) ([][]float32, error) {
	if len(vectors) == 0 {
		return vectors, nil
	}

	dimension := len(vectors[0])
	if dimension == 0 {
		return nil, errors.New("embedding provider returned an empty vector")
	}
	for i, vector := range vectors {
		if len(vector) != dimension {
			return nil, fmt.Errorf("embedding %d has dimension %d; expected %d", i, len(vector), dimension)
		}
		for _, value := range vector {
			if value != value || value > 3.4028235e+38 || value < -3.4028235e+38 {
				return nil, fmt.Errorf("embedding %d contains a non-finite value", i)
			}
		}
	}

	return vectors, nil
}
