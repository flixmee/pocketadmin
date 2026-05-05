package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("errorStepIndex") != nil {
			return nil
		}

		col.Fields.Add(&core.NumberField{
			Name:    "errorStepIndex",
			System:  true,
			OnlyInt: true,
		})

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("errorStepIndex") == nil {
			return nil
		}

		col.Fields.RemoveByName("errorStepIndex")

		return txApp.Save(col)
	})
}
