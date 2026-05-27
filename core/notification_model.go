package core

import (
	"context"
	"errors"
	"strings"

	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameNotifications = "_notifications"

	NotificationSeverityInfo    = "info"
	NotificationSeveritySuccess = "success"
	NotificationSeverityWarning = "warning"
	NotificationSeverityDanger  = "danger"
)

var (
	_ Model        = (*Notification)(nil)
	_ PreValidator = (*Notification)(nil)
	_ RecordProxy  = (*Notification)(nil)
)

// Notification defines a Record proxy for working with the notifications collection.
type Notification struct {
	*Record
}

// NotificationCreateOptions defines the normalized input for creating a notification.
type NotificationCreateOptions struct {
	RecipientCollection string
	RecipientRef        string
	Title               string
	Message             string
	Type                string
	Severity            string
	ActionURL           string
	SourceCollection    string
	SourceRecord        string
	Data                any
	ExpiresAt           types.DateTime
}

// NewNotification instantiates and returns a new blank *Notification model.
func NewNotification(app App) *Notification {
	m := &Notification{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameNotifications)
	if err != nil {
		c = NewBaseCollection("@__invalid_notifications__")
	}

	m.Record = NewRecord(c)

	return m
}

// PreValidate implements the [PreValidator] interface and checks
// whether the proxy is properly loaded.
func (m *Notification) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameNotifications {
		return errors.New("missing or invalid Notification ProxyRecord")
	}

	return nil
}

// ProxyRecord returns the proxied Record model.
func (m *Notification) ProxyRecord() *Record {
	return m.Record
}

// SetProxyRecord loads the specified record model into the current proxy.
func (m *Notification) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Notification) RecipientCollection() string {
	return m.GetString("recipientCollection")
}

func (m *Notification) SetRecipientCollection(collectionIdOrName string) {
	m.Set("recipientCollection", strings.TrimSpace(collectionIdOrName))
}

func (m *Notification) RecipientRef() string {
	return m.GetString("recipientRef")
}

func (m *Notification) SetRecipientRef(recordId string) {
	m.Set("recipientRef", strings.TrimSpace(recordId))
}

func (m *Notification) Title() string {
	return m.GetString("title")
}

func (m *Notification) SetTitle(title string) {
	m.Set("title", strings.TrimSpace(title))
}

func (m *Notification) Message() string {
	return m.GetString("message")
}

func (m *Notification) SetMessage(message string) {
	m.Set("message", strings.TrimSpace(message))
}

func (m *Notification) Type() string {
	return m.GetString("type")
}

func (m *Notification) SetType(notificationType string) {
	m.Set("type", strings.TrimSpace(notificationType))
}

func (m *Notification) Severity() string {
	return m.GetString("severity")
}

func (m *Notification) SetSeverity(severity string) {
	m.Set("severity", strings.TrimSpace(severity))
}

func (m *Notification) Read() bool {
	return m.GetBool("read")
}

func (m *Notification) SetRead(read bool) {
	m.Set("read", read)
}

func (m *Notification) ReadAt() types.DateTime {
	return m.GetDateTime("readAt")
}

func (m *Notification) Archived() bool {
	return m.GetBool("archived")
}

func (m *Notification) SetArchived(archived bool) {
	m.Set("archived", archived)
}

func (m *Notification) ActionURL() string {
	return m.GetString("actionUrl")
}

func (m *Notification) SetActionURL(actionURL string) {
	m.Set("actionUrl", strings.TrimSpace(actionURL))
}

func (m *Notification) SourceCollection() string {
	return m.GetString("sourceCollection")
}

func (m *Notification) SetSourceCollection(collectionIdOrName string) {
	m.Set("sourceCollection", strings.TrimSpace(collectionIdOrName))
}

func (m *Notification) SourceRecord() string {
	return m.GetString("sourceRecord")
}

func (m *Notification) SetSourceRecord(recordId string) {
	m.Set("sourceRecord", strings.TrimSpace(recordId))
}

func (m *Notification) Data() types.JSONRaw {
	raw, _ := m.GetRaw("data").(types.JSONRaw)
	return raw
}

func (m *Notification) SetData(data types.JSONRaw) {
	m.Set("data", data)
}

func (m *Notification) ExpiresAt() types.DateTime {
	return m.GetDateTime("expiresAt")
}

func (m *Notification) Created() types.DateTime {
	return m.GetDateTime("created")
}

func (m *Notification) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
