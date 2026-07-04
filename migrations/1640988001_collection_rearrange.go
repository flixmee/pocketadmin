package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		var total int

		err := txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collections') WHERE name = 'rearrange'").
			Row(&total)
		if err != nil {
			return err
		}

		if total > 0 {
			return nil
		}

		_, err = txApp.DB().AddColumn("_collections", "rearrange", `JSON DEFAULT "{}" NOT NULL`).Execute()
		return err
	}, func(txApp core.App) error {
		var total int

		err := txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collections') WHERE name = 'rearrange'").
			Row(&total)
		if err != nil {
			return err
		}

		if total == 0 {
			return nil
		}

		_, err = txApp.DB().DropColumn("_collections", "rearrange").Execute()
		return err
	})
}
