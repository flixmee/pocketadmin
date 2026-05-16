package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		return createCapabilitiesCollection(txApp)
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_capabilities}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] = {:name}").
			Bind(map[string]any{"name": core.CollectionNameCapabilities}).
			Execute()

		return err
	})
}

func createCapabilitiesCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameCapabilities); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameCapabilities)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "key", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "version", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "category", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "icon", System: true})
	col.Fields.Add(&core.JSONField{Name: "inputSchema", System: true, Required: true})
	col.Fields.Add(&core.JSONField{Name: "outputSchema", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "authStrategy", System: true})
	col.Fields.Add(&core.TextField{Name: "runtimeHandler", System: true, Required: true})
	col.Fields.Add(&core.JSONField{Name: "configUI", System: true})
	col.Fields.Add(&core.BoolField{Name: "active", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})

	col.AddIndex("idx_capabilities_key_version", true, "`key`, `version`", "")
	col.AddIndex("idx_capabilities_active_key", false, "`active`, `key`", "")

	return txApp.Save(col)
}
