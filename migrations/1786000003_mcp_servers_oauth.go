package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		columns := []string{
			`[[kind]] TEXT DEFAULT "http" NOT NULL`,
			`[[clientId]] TEXT DEFAULT "" NOT NULL`,
			`[[clientSecret]] TEXT DEFAULT "" NOT NULL`,
			`[[oauth]] JSON DEFAULT "{}" NOT NULL`,
		}

		for _, column := range columns {
			_, err := txApp.DB().NewQuery("ALTER TABLE {{" + core.MCPServersTableName + "}} ADD COLUMN " + column).Execute()
			if err != nil {
				return err
			}
		}

		return nil
	}, func(txApp core.App) error {
		for _, column := range []string{"kind", "clientId", "clientSecret", "oauth"} {
			_, err := txApp.DB().NewQuery("ALTER TABLE {{" + core.MCPServersTableName + "}} DROP COLUMN [[" + column + "]]").Execute()
			if err != nil {
				return err
			}
		}

		return nil
	})
}
