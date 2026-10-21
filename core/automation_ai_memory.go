package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const AIMemoryTableName = "_aiMemory"

const (
	AutomationAIMemoryDefaultKey         = "default"
	AutomationAIMemoryDefaultMaxMessages = 20
	AutomationAIMemoryMaxMessages        = 100

	automationAIMemoryMaxKeyLength     = 255
	automationAIMemoryMaxContentLength = 20000
)

// automationAIMemoryConfig is the parsed AI generate step "memory" option.
type automationAIMemoryConfig struct {
	Enabled     bool
	Key         any
	MaxMessages int
}

func automationAIMemoryOption(step map[string]any) (*automationAIMemoryConfig, error) {
	raw, ok := step["memory"]
	if !ok || raw == nil {
		return nil, nil
	}

	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("AI step memory must be a JSON object")
	}

	config := &automationAIMemoryConfig{
		Enabled:     obj["enabled"] == true,
		Key:         obj["key"],
		MaxMessages: AutomationAIMemoryDefaultMaxMessages,
	}

	if v, ok := obj["maxMessages"]; ok && v != nil {
		config.MaxMessages = int(toFloat(v))
		if config.MaxMessages < 1 || config.MaxMessages > AutomationAIMemoryMaxMessages {
			return nil, fmt.Errorf("AI step memory maxMessages must be between 1 and %d", AutomationAIMemoryMaxMessages)
		}
	}

	if !config.Enabled {
		return nil, nil
	}

	return config, nil
}

// normalizeAutomationAIMemoryKey converts the rendered memory key to a string.
func normalizeAutomationAIMemoryKey(rendered any) string {
	var key string
	switch v := rendered.(type) {
	case nil:
	case string:
		key = v
	case float64:
		key = toString(v)
	default:
		if raw, err := json.Marshal(v); err == nil {
			key = string(raw)
		}
	}

	key = strings.TrimSpace(key)
	if key == "" {
		key = AutomationAIMemoryDefaultKey
	}
	if len(key) > automationAIMemoryMaxKeyLength {
		key = key[:automationAIMemoryMaxKeyLength]
	}

	return key
}

// automationAIMemoryText converts the AI step input/output to the text stored in the memory.
func automationAIMemoryText(value any) string {
	var text string
	switch v := value.(type) {
	case string:
		text = v
	default:
		if raw, err := json.Marshal(v); err == nil {
			text = string(raw)
		} else {
			text = fmt.Sprint(v)
		}
	}

	if len(text) > automationAIMemoryMaxContentLength {
		text = text[:automationAIMemoryMaxContentLength]
	}

	return text
}

// LoadAutomationAIMemory returns the last limit messages (oldest first) of
// the specified automation memory session.
//
// Leading assistant messages are skipped so that the history always starts
// with a user message (required by some providers).
func LoadAutomationAIMemory(app App, automationId string, key string, limit int) ([]AutomationAIMessage, error) {
	rows := []struct {
		Role    string `db:"role"`
		Content string `db:"content"`
	}{}

	err := app.DB().Select("role", "content").
		From(AIMemoryTableName).
		AndWhere(dbx.HashExp{"automationId": automationId, "sessionKey": key}).
		OrderBy("rowid DESC"). // insertion order
		Limit(int64(limit)).
		All(&rows)
	if err != nil {
		return nil, err
	}

	messages := make([]AutomationAIMessage, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		if len(messages) == 0 && rows[i].Role != "user" {
			continue
		}
		messages = append(messages, AutomationAIMessage{Role: rows[i].Role, Content: rows[i].Content})
	}

	return messages, nil
}

// SaveAutomationAIMemoryTurn appends a user/assistant turn to the memory session
// and removes the older messages exceeding keep.
func SaveAutomationAIMemoryTurn(app App, automationId string, key string, userText string, assistantText string, keep int) error {
	return app.RunInTransaction(func(txApp App) error {
		now := types.NowDateTime().String()
		for _, m := range []AutomationAIMessage{{Role: "user", Content: userText}, {Role: "assistant", Content: assistantText}} {
			_, err := txApp.NonconcurrentDB().Insert(AIMemoryTableName, dbx.Params{
				"id":           security.RandomString(15),
				"automationId": automationId,
				"sessionKey":   key,
				"role":         m.Role,
				"content":      m.Content,
				"created":      now,
			}).Execute()
			if err != nil {
				return err
			}
		}

		_, err := txApp.NonconcurrentDB().NewQuery(`
			DELETE FROM {{` + AIMemoryTableName + `}}
			WHERE [[automationId]] = {:automationId} AND [[sessionKey]] = {:key} AND [[id]] NOT IN (
				SELECT [[id]] FROM {{` + AIMemoryTableName + `}}
				WHERE [[automationId]] = {:automationId} AND [[sessionKey]] = {:key}
				ORDER BY [[rowid]] DESC
				LIMIT {:keep}
			)
		`).Bind(dbx.Params{"automationId": automationId, "key": key, "keep": keep}).Execute()

		return err
	})
}

// AutomationAIMemorySession summarizes a single memory session.
type AutomationAIMemorySession struct {
	Key      string         `db:"sessionKey" json:"key"`
	Messages int            `db:"messages" json:"messages"`
	Updated  types.DateTime `db:"updated" json:"updated"`
}

// FindAutomationAIMemorySessions returns the memory sessions of the automation (most recent first).
func FindAutomationAIMemorySessions(app App, automationId string) ([]AutomationAIMemorySession, error) {
	sessions := []AutomationAIMemorySession{}

	err := app.DB().Select("sessionKey", "COUNT(*) AS messages", "MAX(created) AS updated").
		From(AIMemoryTableName).
		AndWhere(dbx.HashExp{"automationId": automationId}).
		GroupBy("sessionKey").
		OrderBy("updated DESC").
		All(&sessions)

	return sessions, err
}

// FindAutomationAIMemoryMessages returns all messages of a memory session (oldest first).
func FindAutomationAIMemoryMessages(app App, automationId string, key string) ([]AutomationAIMessage, error) {
	return LoadAutomationAIMemory(app, automationId, key, AutomationAIMemoryMaxMessages)
}

// DeleteAutomationAIMemory deletes the automation memory
// (only the specified session when key is not empty).
func DeleteAutomationAIMemory(app App, automationId string, key string) error {
	where := dbx.HashExp{"automationId": automationId}
	if key != "" {
		where["sessionKey"] = key
	}

	_, err := app.NonconcurrentDB().Delete(AIMemoryTableName, where).Execute()

	return err
}

func (app *BaseApp) registerAutomationAIMemoryHooks() {
	app.OnRecordAfterDeleteSuccess(CollectionNameAutomations).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := DeleteAutomationAIMemory(e.App, e.Record.Id, ""); err != nil {
				e.App.Logger().Warn("Failed to delete the automation AI memory", "automationId", e.Record.Id, "error", err)
			}

			return e.Next()
		},
	})
}
