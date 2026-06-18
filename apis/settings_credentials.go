package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
)

var (
	telegramAPIBaseURL  = "https://api.telegram.org"
	googleOAuthTokenURL = "https://oauth2.googleapis.com/token"
)

type settingsTelegramTestRequest struct {
	BaseURL     string  `form:"baseURL" json:"baseURL"`
	AccessToken *string `form:"accessToken" json:"accessToken"`
}

type settingsTelegramRegisterWebhookRequest struct {
	WebhookURL string `form:"webhookURL" json:"webhookURL"`
}

type telegramWebhookRegistrationResult struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      any    `json:"result,omitempty"`
	StatusCode  int    `json:"statusCode"`
	WebhookURL  string `json:"webhookURL"`
}

type settingsGoogleSheetsTestRequest struct {
	OAuthRedirectURL string  `form:"oauthRedirectURL" json:"oauthRedirectURL"`
	ClientID         string  `form:"clientID" json:"clientID"`
	ClientSecret     *string `form:"clientSecret" json:"clientSecret"`
}

func settingsTestTelegram(e *core.RequestEvent) error {
	form := new(settingsTelegramTestRequest)
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("An error occurred while loading the submitted data.", err)
	}

	config := resolveTelegramTestConfig(e.App.Settings().Credentials.Telegram, form)
	if err := testTelegramCredentials(e.Request.Context(), http.DefaultClient, config); err != nil {
		if fErr, ok := err.(validation.Errors); ok {
			return e.BadRequestError("Failed to test Telegram credentials.", fErr)
		}

		return e.BadRequestError("Failed to test Telegram credentials. Raw error: \n"+err.Error(), nil)
	}

	return e.NoContent(http.StatusNoContent)
}

func settingsRegisterTelegramWebhook(e *core.RequestEvent) error {
	form := new(settingsTelegramRegisterWebhookRequest)
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("An error occurred while loading the submitted data.", err)
	}

	result, err := registerTelegramWebhook(
		e.Request.Context(),
		http.DefaultClient,
		e.App.Settings().Credentials.Telegram,
		form.WebhookURL,
	)
	if err != nil {
		if fErr, ok := err.(validation.Errors); ok {
			return e.BadRequestError("Failed to register Telegram webhook.", fErr)
		}

		return e.BadRequestError("Failed to register Telegram webhook. Raw error: \n"+err.Error(), nil)
	}

	return e.JSON(http.StatusOK, result)
}

func settingsTestGoogleSheets(e *core.RequestEvent) error {
	form := new(settingsGoogleSheetsTestRequest)
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("An error occurred while loading the submitted data.", err)
	}

	config := resolveGoogleSheetsTestConfig(e.App.Settings().Credentials.GoogleSheets, form)
	if err := testGoogleSheetsCredentials(e.Request.Context(), http.DefaultClient, googleOAuthTokenURL, config); err != nil {
		if fErr, ok := err.(validation.Errors); ok {
			return e.BadRequestError("Failed to test Google Sheets credentials.", fErr)
		}

		return e.BadRequestError("Failed to test Google Sheets credentials. Raw error: \n"+err.Error(), nil)
	}

	return e.NoContent(http.StatusNoContent)
}

func resolveTelegramTestConfig(
	stored core.TelegramCredentialsConfig,
	form *settingsTelegramTestRequest,
) core.TelegramCredentialsConfig {
	config := stored
	config.Enabled = true

	if form.BaseURL != "" {
		config.BaseURL = form.BaseURL
	}
	if form.AccessToken != nil {
		config.AccessToken = *form.AccessToken
	}

	return config
}

func resolveGoogleSheetsTestConfig(
	stored core.GoogleSheetsCredentialsConfig,
	form *settingsGoogleSheetsTestRequest,
) core.GoogleSheetsCredentialsConfig {
	config := stored
	config.Enabled = true

	if form.OAuthRedirectURL != "" {
		config.OAuthRedirectURL = form.OAuthRedirectURL
	}
	if form.ClientID != "" {
		config.ClientID = form.ClientID
	}
	if form.ClientSecret != nil {
		config.ClientSecret = *form.ClientSecret
	}

	return config
}

func testTelegramCredentials(
	ctx context.Context,
	client *http.Client,
	config core.TelegramCredentialsConfig,
) error {
	if client == nil {
		client = http.DefaultClient
	}
	if config.BaseURL == "" {
		config.BaseURL = telegramAPIBaseURL
	}
	if err := config.Validate(); err != nil {
		return err
	}

	clientWithTimeout := *client
	if clientWithTimeout.Timeout == 0 {
		clientWithTimeout.Timeout = 15 * time.Second
	}

	endpoint := strings.TrimRight(config.BaseURL, "/") + "/bot" + url.PathEscape(config.AccessToken) + "/getMe"
	return sendTelegramTestRequest(ctx, &clientWithTimeout, endpoint)
}

func registerTelegramWebhook(
	ctx context.Context,
	client *http.Client,
	config core.TelegramCredentialsConfig,
	webhookBaseURL string,
) (*telegramWebhookRegistrationResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if config.BaseURL == "" {
		config.BaseURL = telegramAPIBaseURL
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	webhookBaseURL = strings.TrimRight(strings.TrimSpace(webhookBaseURL), "/")
	parsed, err := url.ParseRequestURI(webhookBaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, validation.Errors{
			"webhookURL": validation.NewError("validation_invalid_url", "Invalid Telegram webhook URL."),
		}
	}

	clientWithTimeout := *client
	if clientWithTimeout.Timeout == 0 {
		clientWithTimeout.Timeout = 15 * time.Second
	}

	webhookURL := webhookBaseURL + "/" + url.PathEscape(config.AccessToken)
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/bot" + url.PathEscape(config.AccessToken) + "/setWebhook"
	body := url.Values{}
	body.Set("url", webhookURL)
	body.Set("allowed_updates", `["message","edited_message","channel_post","edited_channel_post"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := clientWithTimeout.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	result := &telegramWebhookRegistrationResult{
		StatusCode: res.StatusCode,
		WebhookURL: webhookBaseURL + "/<access-token>",
	}
	if err := json.NewDecoder(res.Body).Decode(result); err != nil {
		return nil, err
	}

	return result, nil
}

func sendTelegramTestRequest(
	ctx context.Context,
	client *http.Client,
	endpoint string,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Telegram API returned %d", res.StatusCode)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("Telegram API returned ok=false")
	}

	return nil
}

func testGoogleSheetsCredentials(
	ctx context.Context,
	client *http.Client,
	tokenURL string,
	config core.GoogleSheetsCredentialsConfig,
) error {
	if client == nil {
		client = http.DefaultClient
	}
	if tokenURL == "" {
		tokenURL = googleOAuthTokenURL
	}
	if err := config.Validate(); err != nil {
		return err
	}

	clientWithTimeout := *client
	if clientWithTimeout.Timeout == 0 {
		clientWithTimeout.Timeout = 15 * time.Second
	}

	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", "pocketbase_credentials_test")
	body.Set("redirect_uri", config.OAuthRedirectURL)
	body.Set("client_id", config.ClientID)
	body.Set("client_secret", config.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := clientWithTimeout.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var result struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(res.Body).Decode(&result)

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	// A valid OAuth client will reject the synthetic auth code as invalid_grant.
	// Other errors, such as invalid_client, indicate that the credentials likely failed.
	if res.StatusCode == http.StatusBadRequest && result.Error == "invalid_grant" {
		return nil
	}

	if result.Error != "" {
		return fmt.Errorf("Google OAuth token endpoint returned %s", result.Error)
	}

	return fmt.Errorf("Google OAuth token endpoint returned %d", res.StatusCode)
}
