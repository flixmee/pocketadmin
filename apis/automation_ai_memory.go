package apis

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// automationAIMemoryList returns the AI memory sessions of an automation
// or, when the "key" query parameter is set, the messages of a single session.
func automationAIMemoryList(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "view", err)
	}

	if key := strings.TrimSpace(e.Request.URL.Query().Get("key")); key != "" {
		messages, err := core.FindAutomationAIMemoryMessages(e.App, automation.Id, key)
		if err != nil {
			return e.InternalServerError("Failed to load the AI memory.", err)
		}

		return e.JSON(http.StatusOK, map[string]any{"key": key, "messages": messages})
	}

	sessions, err := core.FindAutomationAIMemorySessions(e.App, automation.Id)
	if err != nil {
		return e.InternalServerError("Failed to load the AI memory.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{"items": sessions})
}

// automationAIMemoryDelete clears the AI memory of an automation
// (only a single session when the "key" query parameter is set).
func automationAIMemoryDelete(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "update", err)
	}

	key := strings.TrimSpace(e.Request.URL.Query().Get("key"))

	if err := core.DeleteAutomationAIMemory(e.App, automation.Id, key); err != nil {
		return e.BadRequestError("Failed to clear the AI memory.", err)
	}

	return e.NoContent(http.StatusNoContent)
}
