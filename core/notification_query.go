package core

import (
	"database/sql"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

// CreateNotification creates and persists a notification from the provided options.
func (app *BaseApp) CreateNotification(options NotificationCreateOptions) (*Notification, error) {
	record := NewNotification(app)
	record.SetRecipientCollection(options.RecipientCollection)
	record.SetRecipientRef(options.RecipientRef)
	record.SetTitle(options.Title)
	record.SetMessage(options.Message)
	record.SetType(options.Type)
	if strings.TrimSpace(options.Severity) == "" {
		record.SetSeverity(NotificationSeverityInfo)
	} else {
		record.SetSeverity(options.Severity)
	}
	record.SetActionURL(options.ActionURL)
	record.SetSourceCollection(options.SourceCollection)
	record.SetSourceRecord(options.SourceRecord)

	if options.Data != nil {
		raw, err := toJSONRaw(options.Data)
		if err != nil {
			return nil, err
		}
		record.SetData(raw)
	}

	if !options.ExpiresAt.IsZero() {
		record.Set("expiresAt", options.ExpiresAt)
	}

	if err := app.Save(record); err != nil {
		return nil, err
	}

	return record, nil
}

// FindNotificationById returns a single Notification model by its id.
func (app *BaseApp) FindNotificationById(id string) (*Notification, error) {
	result := &Notification{}

	err := app.RecordQuery(CollectionNameNotifications).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindNotificationsByRecipient returns recent, non-archived notifications for the provided recipient.
func (app *BaseApp) FindNotificationsByRecipient(recipientCollection string, recipientRef string, limit int) ([]*Notification, error) {
	result := []*Notification{}

	query := app.RecordQuery(CollectionNameNotifications).
		AndWhere(notificationRecipientExpression(recipientCollection, recipientRef)).
		AndWhere(dbx.HashExp{"archived": false}).
		OrderBy("created DESC")

	if limit > 0 {
		query.Limit(int64(limit))
	}

	err := query.All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CountUnreadNotifications returns the total unread, non-archived notifications for the provided recipient.
func (app *BaseApp) CountUnreadNotifications(recipientCollection string, recipientRef string) (int, error) {
	total := 0

	err := app.RecordQuery(CollectionNameNotifications).
		Select("count(*)").
		AndWhere(notificationRecipientExpression(recipientCollection, recipientRef)).
		AndWhere(dbx.HashExp{"read": false, "archived": false}).
		Row(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

// MarkNotificationRead marks a single recipient notification as read.
func (app *BaseApp) MarkNotificationRead(recipientCollection string, recipientRef string, id string) (*Notification, error) {
	notification, err := app.FindNotificationById(id)
	if err != nil {
		return nil, err
	}

	if !isNotificationRecipient(notification, recipientCollection, recipientRef) {
		return nil, sql.ErrNoRows
	}

	if !notification.Read() {
		notification.SetRead(true)
		notification.Set("readAt", types.NowDateTime())
		if err := app.Save(notification); err != nil {
			return nil, err
		}
	}

	return notification, nil
}

// MarkAllNotificationsRead marks all unread, non-archived notifications for the provided recipient as read.
func (app *BaseApp) MarkAllNotificationsRead(recipientCollection string, recipientRef string) (int, error) {
	notifications := []*Notification{}
	err := app.RecordQuery(CollectionNameNotifications).
		AndWhere(notificationRecipientExpression(recipientCollection, recipientRef)).
		AndWhere(dbx.HashExp{"read": false, "archived": false}).
		All(&notifications)
	if err != nil {
		return 0, err
	}

	for _, notification := range notifications {
		notification.SetRead(true)
		notification.Set("readAt", types.NowDateTime())
		if err := app.Save(notification); err != nil {
			return 0, err
		}
	}

	return len(notifications), nil
}

func notificationRecipientExpression(recipientCollection string, recipientRef string) dbx.Expression {
	recipientCollection = strings.TrimSpace(recipientCollection)
	recipientRef = strings.TrimSpace(recipientRef)

	return dbx.HashExp{
		"recipientCollection": recipientCollection,
		"recipientRef":        recipientRef,
	}
}

func isNotificationRecipient(notification *Notification, recipientCollection string, recipientRef string) bool {
	if notification == nil {
		return false
	}

	return notification.RecipientCollection() == strings.TrimSpace(recipientCollection) &&
		notification.RecipientRef() == strings.TrimSpace(recipientRef)
}
