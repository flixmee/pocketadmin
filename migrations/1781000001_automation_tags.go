package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("tag") == nil {
			col.Fields.Add(&core.TextField{Name: "tag", System: true})
		}

		col.AddIndex("idx_automations_tag", false, "`tag`", "`tag` != ''")

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations)
		if err != nil {
			return nil
		}

		col.RemoveIndex("idx_automations_tag")
		col.Fields.RemoveByName("tag")

		return txApp.Save(col)
	})
}
