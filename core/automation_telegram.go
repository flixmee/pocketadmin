package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/tools/routine"
)

// QueueAutomationTelegramMessage queues all active Telegram message automations
// that match a message-like Telegram update.
func (app *BaseApp) QueueAutomationTelegramMessage(update map[string]any) (int, error) {
	message, messageType := automationTelegramMessage(update)
	if message == nil {
		return 0, nil
	}

	registry, err := getAutomationRegistry(app)
	if err != nil {
		return 0, err
	}

	automations := registry.ByTrigger[AutomationTriggerTelegramMessage]
	if len(automations) == 0 {
		return 0, nil
	}

	payload := automationTelegramTriggerPayload(update, message, messageType)
	inheritAutomationPolicyContext(app, &payload)
	_, _ = app.PublishAutomationEvent(AutomationEventEnvelope{
		Name:    AutomationTriggerTelegramMessage,
		Source:  "telegram",
		Subject: automationTelegramSubject(payload.Telegram),
		Payload: map[string]any{
			"telegram": payload.Telegram,
		},
	})

	queued := 0
	for _, automation := range automations {
		if automation == nil {
			continue
		}
		queued++

		nextAutomation := automation
		nextPayload := payload
		routine.FireAndForget(func() {
			if err := runAutomation(app, nextAutomation, nextPayload); err != nil {
				app.Logger().Warn(
					"Failed to execute Telegram message automation",
					"automationId", nextAutomation.Id,
					"error", err,
				)
			}
		})
	}

	return queued, nil
}

func executeAutomationTelegramStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	credentials := ctx.App.Settings().Credentials.Telegram
	if !credentials.Enabled {
		return nil, fmt.Errorf("Telegram credentials are not enabled")
	}

	if strings.TrimSpace(credentials.BaseURL) == "" {
		credentials.BaseURL = "https://api.telegram.org"
	}
	if strings.TrimSpace(credentials.AccessToken) == "" {
		return nil, fmt.Errorf("Telegram access token is required")
	}

	chatId, err := renderAutomationTelegramString(ctx, step["chatId"])
	if err != nil {
		return nil, err
	}
	chatId = strings.TrimSpace(chatId)
	if chatId == "" {
		return nil, fmt.Errorf("Telegram step requires a chat ID")
	}

	text, err := renderAutomationTelegramString(ctx, step["text"])
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("Telegram step requires a message")
	}

	parseMode, err := renderAutomationTelegramString(ctx, step["parseMode"])
	if err != nil {
		return nil, err
	}
	parseMode = strings.TrimSpace(parseMode)

	messageId, err := sendAutomationTelegramMessage(credentials, chatId, text, parseMode, automationTelegramBool(step["disableWebPagePreview"]))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sent":      true,
		"chatId":    chatId,
		"messageId": messageId,
	}, nil
}

func renderAutomationTelegramString(ctx *automationExecutionContext, raw any) (string, error) {
	if raw == nil {
		return "", nil
	}

	rendered, err := renderAutomationTemplateValue(raw, ctx.TemplateData)
	if err != nil {
		return "", err
	}

	return toString(rendered), nil
}

func sendAutomationTelegramMessage(
	credentials TelegramCredentialsConfig,
	chatId string,
	text string,
	parseMode string,
	disableWebPagePreview bool,
) (int, error) {
	body := url.Values{}
	body.Set("chat_id", chatId)
	body.Set("text", text)
	if parseMode != "" {
		body.Set("parse_mode", parseMode)
	}
	if disableWebPagePreview {
		body.Set("disable_web_page_preview", "true")
	}

	endpoint := strings.TrimRight(credentials.BaseURL, "/") + "/bot" + url.PathEscape(credentials.AccessToken) + "/sendMessage"
	reqCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 || !result.OK {
		if result.Description != "" {
			return 0, fmt.Errorf("Telegram API error: %s", result.Description)
		}
		return 0, fmt.Errorf("Telegram API returned %d", res.StatusCode)
	}

	return result.Result.MessageID, nil
}

func automationTelegramBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func automationTelegramMessage(update map[string]any) (map[string]any, string) {
	for _, key := range []string{"message", "edited_message", "channel_post", "edited_channel_post"} {
		message, ok := update[key].(map[string]any)
		if ok && len(message) > 0 {
			return message, key
		}
	}

	return nil, ""
}

func automationTelegramTriggerPayload(update map[string]any, message map[string]any, messageType string) automationTriggerPayload {
	telegram := map[string]any{
		"update":      update,
		"message":     message,
		"messageType": messageType,
	}

	if value := toString(update["update_id"]); strings.TrimSpace(value) != "" {
		telegram["updateId"] = strings.TrimSpace(value)
	}
	if chat, ok := message["chat"].(map[string]any); ok {
		telegram["chat"] = chat
	}
	if from, ok := message["from"].(map[string]any); ok {
		telegram["from"] = from
	}
	if text := strings.TrimSpace(toString(message["text"])); text != "" {
		telegram["text"] = text
	} else if caption := strings.TrimSpace(toString(message["caption"])); caption != "" {
		telegram["text"] = caption
	}

	return automationTriggerPayload{
		TriggerType: AutomationTriggerTelegramMessage,
		Telegram:    telegram,
	}
}

func automationTelegramSubject(telegram map[string]any) string {
	if chat, ok := telegram["chat"].(map[string]any); ok {
		if title := strings.TrimSpace(toString(chat["title"])); title != "" {
			return title
		}
		if username := strings.TrimSpace(toString(chat["username"])); username != "" {
			return username
		}
		if id := strings.TrimSpace(toString(chat["id"])); id != "" {
			return id
		}
	}

	return strings.TrimSpace(toString(telegram["updateId"]))
}
