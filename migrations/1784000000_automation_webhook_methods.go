package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("webhookMethod") == nil {
			col.Fields.Add(&core.TextField{Name: "webhookMethod", System: true})
		}

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		col.Fields.RemoveByName("webhookMethod")

		return txApp.Save(col)
	})
}
