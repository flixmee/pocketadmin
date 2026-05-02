package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameMedias); err == nil {
			return nil
		}

		return createMediasCollection(txApp)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameMedias)
		if err != nil {
			return nil
		}

		if err := txApp.Delete(col); err != nil {
			return err
		}

		return nil
	})
}
