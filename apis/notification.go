package apis

import (
	"net/http"
	"strconv"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// bindNotificationApi registers the notification api endpoints.
func bindNotificationApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	subGroup := rg.Group("/notifications").Bind(RequireAuth())
	subGroup.GET("", notificationsList)
	subGroup.GET("/unread-count", notificationsUnreadCount)
	subGroup.POST("/read", notificationsMarkAllRead)
	subGroup.POST("/{id}/read", notificationsMarkRead)
}

func notificationsList(e *core.RequestEvent) error {
	recipientCollection, recipientRef := notificationRecipientFromAuth(e)
	limit := 20
	if raw := e.Request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return e.BadRequestError("Invalid notifications limit.", err)
		}
		limit = parsed
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	notifications, err := e.App.FindNotificationsByRecipient(recipientCollection, recipientRef, limit)
	if err != nil {
		return e.BadRequestError("Failed to load notifications.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, notifications)
	})
}

func notificationsUnreadCount(e *core.RequestEvent) error {
	recipientCollection, recipientRef := notificationRecipientFromAuth(e)

	count, err := e.App.CountUnreadNotifications(recipientCollection, recipientRef)
	if err != nil {
		return e.BadRequestError("Failed to load unread notifications count.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, map[string]int{"count": count})
	})
}

func notificationsMarkRead(e *core.RequestEvent) error {
	recipientCollection, recipientRef := notificationRecipientFromAuth(e)

	notification, err := e.App.MarkNotificationRead(recipientCollection, recipientRef, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing notification.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, notification)
	})
}

func notificationsMarkAllRead(e *core.RequestEvent) error {
	recipientCollection, recipientRef := notificationRecipientFromAuth(e)

	count, err := e.App.MarkAllNotificationsRead(recipientCollection, recipientRef)
	if err != nil {
		return e.BadRequestError("Failed to mark notifications as read.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, map[string]int{"count": count})
	})
}

func notificationRecipientFromAuth(e *core.RequestEvent) (string, string) {
	return e.Auth.Collection().Id, e.Auth.Id
}
