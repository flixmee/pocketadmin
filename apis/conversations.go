package apis

import (
	"errors"
	"net/http"
	"strconv"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// bindConversationsApi registers the admin conversations (inbox) routes.
//
// Zalo is the only channel for now - other channels are expected to
// register their own /api/conversations/{channel} routes and an entry
// in conversationChannels.
func bindConversationsApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	sub := rg.Group("/conversations").Bind(RequireSuperuserAuth())
	sub.GET("/channels", conversationChannels)
	sub.GET("/zalo", zaloConversationsList)
	sub.GET("/zalo/{threadId}/messages", zaloConversationMessagesList)
	sub.POST("/zalo/messages", zaloConversationSend)
}

// conversationChannels lists the conversation channels and whether they
// can be used (the admin UI shows the Conversations menu only for active ones).
func conversationChannels(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, []map[string]any{zaloChannelState(e.App)})
}

// zaloChannelState reports whether Zalo is enabled and the account is logged in.
func zaloChannelState(app core.App) map[string]any {
	config := app.Settings().Credentials.Zalo

	state := map[string]any{
		"id":      "zalo",
		"name":    "Zalo",
		"enabled": config.Enabled,
		"active":  false,
		"status":  "disabled",
	}
	if !config.Enabled {
		return state
	}

	if _, err := app.FindCachedCollectionByNameOrId(core.ZaloCollectionMessages); err != nil {
		state["status"] = "missing_collections"
		return state
	}

	state["status"] = "logged_out"
	if session, err := app.FindRecordById(core.ZaloCollectionSession, core.ZaloSessionRecordId); err == nil {
		state["status"] = session.GetString("status")
		state["accountId"] = session.GetString("account_id")
		state["accountName"] = session.GetString("account_name")
	}

	state["active"] = state["status"] == "logged_in"

	return state
}

func zaloConversationsAvailable(e *core.RequestEvent) error {
	if !e.App.Settings().Credentials.Zalo.Enabled {
		return e.BadRequestError("The Zalo integration is not enabled.", nil)
	}
	for _, name := range core.ZaloCollectionNames {
		if _, err := e.App.FindCachedCollectionByNameOrId(name); err != nil {
			return e.BadRequestError("The Zalo collections are missing - save the Zalo settings to create them.", nil)
		}
	}
	return nil
}

func zaloConversationsList(e *core.RequestEvent) error {
	if err := zaloConversationsAvailable(e); err != nil {
		return err
	}

	query := e.Request.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))

	conversations, err := core.ZaloConversations(e.App, core.ZaloConversationsQuery{
		Search: query.Get("search"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return e.BadRequestError("Failed to load the Zalo conversations.", err)
	}

	return e.JSON(http.StatusOK, conversations)
}

func zaloConversationMessagesList(e *core.RequestEvent) error {
	if err := zaloConversationsAvailable(e); err != nil {
		return err
	}

	query := e.Request.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))

	messages, hasMore, err := core.ZaloConversationMessages(
		e.App,
		e.Request.PathValue("threadId"),
		query.Get("before"),
		limit,
	)
	if err != nil {
		return e.BadRequestError("Failed to load the Zalo messages.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"items":   messages,
		"hasMore": hasMore,
	})
}

func zaloConversationSend(e *core.RequestEvent) error {
	if err := zaloConversationsAvailable(e); err != nil {
		return err
	}

	form := core.ZaloSendForm{}
	if err := e.BindBody(&form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	message, err := core.ZaloSendMessage(e.App, form)
	if err != nil {
		var validationErrs validation.Errors
		if errors.As(err, &validationErrs) {
			return e.BadRequestError("Failed to send the Zalo message.", validationErrs)
		}
		return e.BadRequestError("Failed to send the Zalo message.", err)
	}

	return e.JSON(http.StatusOK, message)
}
