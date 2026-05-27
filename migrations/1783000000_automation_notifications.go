package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("notifyOnCompletion") == nil {
			col.Fields.Add(&core.BoolField{Name: "notifyOnCompletion", System: true})
		}

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		col.Fields.RemoveByName("notifyOnCompletion")

		return txApp.Save(col)
	})
}
