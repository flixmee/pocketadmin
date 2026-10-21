package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		_, err := txApp.DB().
			NewQuery(`
				CREATE TABLE IF NOT EXISTS {{` + core.MCPServersTableName + `}} (
					[[id]]        TEXT PRIMARY KEY NOT NULL,
					[[name]]      TEXT DEFAULT "" NOT NULL,
					[[url]]       TEXT DEFAULT "" NOT NULL,
					[[authToken]] TEXT DEFAULT "" NOT NULL,
					[[headers]]   JSON DEFAULT "{}" NOT NULL,
					[[active]]    BOOLEAN DEFAULT TRUE NOT NULL,
					[[created]]   TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
					[[updated]]   TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
				)
			`).
			Execute()

		return err
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{" + core.MCPServersTableName + "}}").Execute()
		return err
	})
}
