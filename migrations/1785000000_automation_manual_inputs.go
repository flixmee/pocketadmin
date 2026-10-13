package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("manualInputs") == nil {
			col.Fields.Add(&core.JSONField{Name: "manualInputs", System: true})
		}

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		col.Fields.RemoveByName("manualInputs")

		return txApp.Save(col)
	})
}
