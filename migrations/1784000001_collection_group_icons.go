package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		var total int

		err := txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collection_groups') WHERE name = 'icon'").
			Row(&total)
		if err != nil {
			return err
		}

		if total > 0 {
			return nil
		}

		_, err = txApp.DB().AddColumn("_collection_groups", "icon", `TEXT DEFAULT "" NOT NULL`).Execute()
		return err
	}, func(txApp core.App) error {
		var total int

		err := txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collection_groups') WHERE name = 'icon'").
			Row(&total)
		if err != nil {
			return err
		}

		if total == 0 {
			return nil
		}

		_, err = txApp.DB().DropColumn("_collection_groups", "icon").Execute()
		return err
	})
}
