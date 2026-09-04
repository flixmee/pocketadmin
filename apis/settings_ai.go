package apis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
)

type settingsAIModelsRequest struct {
	Provider string  `form:"provider" json:"provider"`
	APIKey   *string `form:"apiKey" json:"apiKey"`
	BaseURL  string  `form:"baseURL" json:"baseURL"`
}

type settingsAIModel struct {
	Id    string `json:"id"`
	Label string `json:"label"`
}

func settingsListAIModels(e *core.RequestEvent) error {
	form := new(settingsAIModelsRequest)
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("An error occurred while loading the submitted data.", err)
	}

	config := resolveAIModelsRequestConfig(e.App.Settings().AI, form)
	models, err := fetchAIModels(e.Request.Context(), http.DefaultClient, config)
	if err != nil {
		if fErr, ok := err.(validation.Errors); ok {
			return e.BadRequestError("Failed to fetch AI models.", fErr)
		}

		return e.BadRequestError("Failed to fetch AI models. Raw error: \n"+err.Error(), nil)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"models": models,
	})
}

func resolveAIModelsRequestConfig(stored core.AIConfig, form *settingsAIModelsRequest) core.AIConfig {
	config := stored

	if form.Provider != "" {
		config.Provider = form.Provider
	}

	if form.APIKey != nil {
		config.APIKey = *form.APIKey
	}

	if form.BaseURL != "" {
		config.BaseURL = form.BaseURL
	}

	if config.Provider == "" {
		config.Provider = core.AIProviderOpenAI
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultAIProviderBaseURL(config.Provider)
	}

	return config
}

func defaultAIProviderBaseURL(provider string) string {
	switch provider {
	case core.AIProviderOpenAI:
		return core.AIProviderOpenAIBaseURL
	case core.AIProviderGemini:
		return core.AIProviderGeminiBaseURL
	case core.AIProviderAnthropic:
		return core.AIProviderAnthropicBaseURL
	default:
		return core.AIProviderCustomBaseURL
	}
}

func fetchAIModels(ctx context.Context, client *http.Client, config core.AIConfig) ([]settingsAIModel, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if err := validateAIModelsRequestConfig(config); err != nil {
		return nil, err
	}

	endpoint, err := aiModelsEndpoint(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	switch config.Provider {
	case core.AIProviderGemini:
		q := req.URL.Query()
		q.Set("key", config.APIKey)
		req.URL.RawQuery = q.Encode()
	case core.AIProviderAnthropic:
		req.Header.Set("x-api-key", config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	clientWithTimeout := *client
	if clientWithTimeout.Timeout == 0 {
		clientWithTimeout.Timeout = 15 * time.Second
	}

	res, err := clientWithTimeout.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("provider returned %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}

	models, err := parseAIModelsResponse(raw)
	if err != nil {
		return nil, err
	}

	if len(models) == 0 {
		return nil, errors.New("provider returned an empty models list")
	}

	sort.SliceStable(models, func(i, j int) bool {
		return strings.ToLower(models[i].Id) < strings.ToLower(models[j].Id)
	})

	return models, nil
}

func validateAIModelsRequestConfig(config core.AIConfig) error {
	return validation.ValidateStruct(&config,
		validation.Field(
			&config.Provider,
			validation.Required,
			validation.In(core.AIProviderOpenAI, core.AIProviderGemini, core.AIProviderAnthropic, core.AIProviderCustom),
		),
		validation.Field(&config.APIKey, validation.Required),
		validation.Field(&config.BaseURL, validation.Required, validation.By(validateAIModelsBaseURL)),
	)
}

func validateAIModelsBaseURL(value any) error {
	v, _ := value.(string)
	parsed, err := url.Parse(v)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return validation.NewError("validation_invalid_url", "Must be a valid URL.")
	}

	return nil
}

func aiModelsEndpoint(config core.AIConfig) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(config.BaseURL, "/") + "/models")
	if err != nil {
		return "", err
	}

	return parsed.String(), nil
}

func parseAIModelsResponse(raw []byte) ([]settingsAIModel, error) {
	var response struct {
		Data []struct {
			Id           string `json:"id"`
			Name         string `json:"name"`
			DisplayName  string `json:"display_name"`
			DisplayName2 string `json:"displayName"`
		} `json:"data"`
		Models []struct {
			Id           string `json:"id"`
			Name         string `json:"name"`
			DisplayName  string `json:"display_name"`
			DisplayName2 string `json:"displayName"`
		} `json:"models"`
	}

	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}

	unique := map[string]settingsAIModel{}
	for _, item := range response.Data {
		id := strings.TrimPrefix(strings.TrimSpace(firstNonEmpty(item.Id, item.Name)), "models/")
		if id == "" {
			continue
		}

		label := firstNonEmpty(item.DisplayName, item.DisplayName2, id)
		unique[id] = settingsAIModel{Id: id, Label: label}
	}
	for _, item := range response.Models {
		id := strings.TrimPrefix(strings.TrimSpace(firstNonEmpty(item.Id, item.Name)), "models/")
		if id == "" {
			continue
		}

		label := firstNonEmpty(item.DisplayName, item.DisplayName2, id)
		unique[id] = settingsAIModel{Id: id, Label: label}
	}

	result := make([]settingsAIModel, 0, len(unique))
	for _, model := range unique {
		result = append(result, model)
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}

	return ""
}
