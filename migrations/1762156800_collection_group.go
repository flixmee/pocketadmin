package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		var total int

		err := txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collections') WHERE name = 'collectionGroup'").
			Row(&total)
		if err != nil {
			return err
		}

		if total > 0 {
			goto ensureGroupsTable
		}

		_, err = txApp.DB().AddColumn("_collections", "collectionGroup", `TEXT DEFAULT "" NOT NULL`).Execute()
		if err != nil {
			return err
		}

ensureGroupsTable:
		_, err = txApp.DB().
			NewQuery(`
				CREATE TABLE IF NOT EXISTS {{_collection_groups}} (
					[[name]]    TEXT PRIMARY KEY NOT NULL,
					[[created]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
					[[updated]] TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
				)
			`).
			Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery(`
				INSERT OR IGNORE INTO {{_collection_groups}} ([[name]], [[created]], [[updated]])
				SELECT DISTINCT TRIM([[collectionGroup]]), (strftime('%Y-%m-%d %H:%M:%fZ')), (strftime('%Y-%m-%d %H:%M:%fZ'))
				FROM {{_collections}}
				WHERE TRIM([[collectionGroup]]) != ''
			`).
			Execute()

		return err
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_collection_groups}}").Execute()
		if err != nil {
			return err
		}

		var total int
		err = txApp.DB().
			NewQuery("SELECT count(*) FROM pragma_table_info('_collections') WHERE name = 'collectionGroup'").
			Row(&total)
		if err != nil {
			return err
		}

		if total == 0 {
			return nil
		}

		_, err = txApp.DB().DropColumn("_collections", "collectionGroup").Execute()

		return err
	})
}
