package core_test

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestNotificationsCollectionSchema(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameNotifications)
	if err != nil {
		t.Fatalf("expected %s collection to exist: %v", core.CollectionNameNotifications, err)
	}

	if !collection.System {
		t.Fatalf("expected %s to be a system collection", core.CollectionNameNotifications)
	}

	expectedFields := []string{
		"id",
		"recipientCollection",
		"recipientRef",
		"title",
		"message",
		"type",
		"severity",
		"read",
		"readAt",
		"archived",
		"actionUrl",
		"sourceCollection",
		"sourceRecord",
		"data",
		"expiresAt",
		"created",
		"updated",
	}
	for _, fieldName := range expectedFields {
		if collection.Fields.GetByName(fieldName) == nil {
			t.Fatalf("missing field %q", fieldName)
		}
	}

	if collection.ListRule == nil || !strings.Contains(*collection.ListRule, "recipientRef = @request.auth.id") {
		t.Fatalf("expected recipient list rule, got %v", collection.ListRule)
	}
	if collection.ViewRule == nil || !strings.Contains(*collection.ViewRule, "recipientCollection = @request.auth.collectionId") {
		t.Fatalf("expected recipient view rule, got %v", collection.ViewRule)
	}
	if collection.CreateRule != nil || collection.UpdateRule != nil || collection.DeleteRule != nil {
		t.Fatal("expected create/update/delete rules to be disabled")
	}

	indexes, err := app.TableIndexes(core.CollectionNameNotifications)
	if err != nil {
		t.Fatal(err)
	}

	for _, indexName := range []string{
		"idx_notifications_recipient_read_created",
		"idx_notifications_recipient_archived_created",
		"idx_notifications_source",
		"idx_notifications_expires",
	} {
		if _, ok := indexes[indexName]; !ok {
			t.Fatalf("missing index %q: %v", indexName, indexes)
		}
	}
}

func TestNotificationHelpers(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	superuser, err := app.FindRecordById(core.CollectionNameSuperusers, "sywbhecnh46rhm0")
	if err != nil {
		t.Fatal(err)
	}

	otherSuperuser, err := app.FindRecordById(core.CollectionNameSuperusers, "sbmbsdb40jyxf7h")
	if err != nil {
		t.Fatal(err)
	}

	first, err := app.CreateNotification(core.NotificationCreateOptions{
		RecipientCollection: superuser.Collection().Id,
		RecipientRef:        superuser.Id,
		Title:               "First notification",
		Severity:            core.NotificationSeverityWarning,
		ActionURL:           "#/logs",
		Data:                map[string]any{"source": "test"},
	})
	if err != nil {
		t.Fatalf("failed to create first notification: %v", err)
	}

	second, err := app.CreateNotification(core.NotificationCreateOptions{
		RecipientCollection: superuser.Collection().Id,
		RecipientRef:        superuser.Id,
		Title:               "Second notification",
	})
	if err != nil {
		t.Fatalf("failed to create second notification: %v", err)
	}

	_, err = app.CreateNotification(core.NotificationCreateOptions{
		RecipientCollection: otherSuperuser.Collection().Id,
		RecipientRef:        otherSuperuser.Id,
		Title:               "Other notification",
	})
	if err != nil {
		t.Fatalf("failed to create other notification: %v", err)
	}

	count, err := app.CountUnreadNotifications(superuser.Collection().Id, superuser.Id)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unread notifications, got %d", count)
	}

	if first.Severity() != core.NotificationSeverityWarning {
		t.Fatalf("expected warning severity, got %q", first.Severity())
	}
	if second.Severity() != core.NotificationSeverityInfo {
		t.Fatalf("expected default info severity, got %q", second.Severity())
	}

	if _, err := app.MarkNotificationRead(superuser.Collection().Id, superuser.Id, first.Id); err != nil {
		t.Fatalf("failed to mark notification read: %v", err)
	}

	refreshed, err := app.FindNotificationById(first.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed.Read() || refreshed.ReadAt().IsZero() {
		t.Fatalf("expected read notification with readAt, got read=%v readAt=%v", refreshed.Read(), refreshed.ReadAt())
	}

	count, err = app.CountUnreadNotifications(superuser.Collection().Id, superuser.Id)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unread notification, got %d", count)
	}

	changed, err := app.MarkAllNotificationsRead(superuser.Collection().Id, superuser.Id)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("expected 1 changed notification, got %d", changed)
	}
}

func TestNotificationValidation(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	_, err = app.CreateNotification(core.NotificationCreateOptions{
		RecipientCollection: core.CollectionNameSuperusers,
		RecipientRef:        "sywbhecnh46rhm0",
		Severity:            "nope",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "severity") || !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected severity and title validation errors, got %v", err)
	}
}
