package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameNotifications); err == nil {
			return nil
		}

		return createNotificationsCollection(txApp)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameNotifications)
		if err != nil {
			return nil
		}

		return txApp.Delete(col)
	})
}

func createNotificationsCollection(txApp core.App) error {
	col := core.NewBaseCollection(core.CollectionNameNotifications)
	col.System = true

	ownerRule := "@request.auth.id != '' && recipientRef = @request.auth.id && recipientCollection = @request.auth.collectionId && archived = false"
	col.ListRule = types.Pointer(ownerRule)
	col.ViewRule = types.Pointer(ownerRule)
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil

	col.Fields.Add(&core.TextField{
		Name:     "recipientCollection",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "recipientRef",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "title",
		System:   true,
		Required: true,
		Max:      255,
	})
	col.Fields.Add(&core.TextField{
		Name:   "message",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "type",
		System: true,
		Max:    80,
	})
	col.Fields.Add(&core.SelectField{
		Name:   "severity",
		System: true,
		Values: []string{
			core.NotificationSeverityInfo,
			core.NotificationSeveritySuccess,
			core.NotificationSeverityWarning,
			core.NotificationSeverityDanger,
		},
	})
	col.Fields.Add(&core.BoolField{
		Name:   "read",
		System: true,
	})
	col.Fields.Add(&core.DateField{
		Name:   "readAt",
		System: true,
	})
	col.Fields.Add(&core.BoolField{
		Name:   "archived",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "actionUrl",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "sourceCollection",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "sourceRecord",
		System: true,
	})
	col.Fields.Add(&core.JSONField{
		Name:   "data",
		System: true,
	})
	col.Fields.Add(&core.DateField{
		Name:   "expiresAt",
		System: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "created",
		System:   true,
		OnCreate: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "updated",
		System:   true,
		OnCreate: true,
		OnUpdate: true,
	})

	col.AddIndex("idx_notifications_recipient_read_created", false, "recipientCollection, recipientRef, read, created", "")
	col.AddIndex("idx_notifications_recipient_archived_created", false, "recipientCollection, recipientRef, archived, created", "")
	col.AddIndex("idx_notifications_source", false, "sourceCollection, sourceRecord", "sourceCollection != '' OR sourceRecord != ''")
	col.AddIndex("idx_notifications_expires", false, "expiresAt", "expiresAt IS NOT NULL")

	return txApp.Save(col)
}
