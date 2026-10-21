package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		_, err := txApp.DB().
			NewQuery(`
				CREATE TABLE IF NOT EXISTS {{` + core.AIMemoryTableName + `}} (
					[[id]]           TEXT PRIMARY KEY NOT NULL,
					[[automationId]] TEXT DEFAULT "" NOT NULL,
					[[sessionKey]]   TEXT DEFAULT "" NOT NULL,
					[[role]]         TEXT DEFAULT "" NOT NULL,
					[[content]]      TEXT DEFAULT "" NOT NULL,
					[[created]]      TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
				)
			`).
			Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("CREATE INDEX IF NOT EXISTS idx_aiMemory_session ON {{" + core.AIMemoryTableName + "}} ([[automationId]], [[sessionKey]], [[created]])").
			Execute()

		return err
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{" + core.AIMemoryTableName + "}}").Execute()
		return err
	})
}
