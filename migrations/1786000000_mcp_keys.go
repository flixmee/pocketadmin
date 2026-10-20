package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		_, err := txApp.DB().
			NewQuery(`
				CREATE TABLE IF NOT EXISTS {{` + core.MCPKeysTableName + `}} (
					[[id]]          TEXT PRIMARY KEY NOT NULL,
					[[name]]        TEXT DEFAULT "" NOT NULL,
					[[keyHash]]     TEXT DEFAULT "" NOT NULL,
					[[keyPrefix]]   TEXT DEFAULT "" NOT NULL,
					[[permissions]] JSON DEFAULT "[]" NOT NULL,
					[[collections]] JSON DEFAULT "[]" NOT NULL,
					[[active]]      BOOLEAN DEFAULT TRUE NOT NULL,
					[[expiresAt]]   TEXT DEFAULT "" NOT NULL,
					[[lastUsedAt]]  TEXT DEFAULT "" NOT NULL,
					[[created]]     TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
					[[updated]]     TEXT DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
				)
			`).
			Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("CREATE UNIQUE INDEX IF NOT EXISTS idx_mcpKeys_keyHash ON {{" + core.MCPKeysTableName + "}} ([[keyHash]])").
			Execute()

		return err
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{" + core.MCPKeysTableName + "}}").Execute()
		return err
	})
}
