package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		return createWorkflowTemplatesCollection(txApp)
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_workflowTemplates}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] = {:name}").
			Bind(map[string]any{"name": core.CollectionNameWorkflowTemplates}).
			Execute()

		return err
	})
}

func createWorkflowTemplatesCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameWorkflowTemplates); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameWorkflowTemplates)
	col.System = true
	col.Fields.Add(&core.TextField{Name: "name", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "description", System: true})
	col.Fields.Add(&core.JSONField{Name: "package", System: true, Required: true})
	col.Fields.Add(&core.JSONField{Name: "requiredCapabilities", System: true})
	col.Fields.Add(&core.JSONField{Name: "requiredConnectors", System: true})
	col.Fields.Add(&core.JSONField{Name: "requiredCollections", System: true})
	col.Fields.Add(&core.BoolField{Name: "active", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	col.AddIndex("idx_workflowTemplates_active_name", false, "`active`, `name`", "")

	return txApp.Save(col)
}
