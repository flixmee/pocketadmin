package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if err := createConnectorsCollection(txApp); err != nil {
			return err
		}
		if err := createAutomationEventsCollection(txApp); err != nil {
			return err
		}
		if err := createWorkflowVersionsCollection(txApp); err != nil {
			return err
		}
		if err := addAutomationVersionFields(txApp); err != nil {
			return err
		}
		return addCapabilityConnectorFields(txApp)
	}, func(txApp core.App) error {
		if col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns); err == nil {
			col.Fields.RemoveByName("workflowVersionRef")
			col.Fields.RemoveByName("workflowVersionSnapshot")
			_ = txApp.Save(col)
		}
		if col, err := txApp.FindCollectionByNameOrId(core.CollectionNameCapabilities); err == nil {
			col.Fields.RemoveByName("connectorRef")
			col.Fields.RemoveByName("requiredScopes")
			_ = txApp.Save(col)
		}

		for _, name := range []string{
			core.CollectionNameWorkflowVersions,
			core.CollectionNameAutomationEvents,
			core.CollectionNameConnectors,
		} {
			if _, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{" + name + "}}").Execute(); err != nil {
				return err
			}
		}

		_, err := txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] IN ({:connectors}, {:events}, {:versions})").
			Bind(map[string]any{
				"connectors": core.CollectionNameConnectors,
				"events":     core.CollectionNameAutomationEvents,
				"versions":   core.CollectionNameWorkflowVersions,
			}).
			Execute()
		return err
	})
}

func createConnectorsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameConnectors); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameConnectors)
	col.System = true
	col.Fields.Add(&core.TextField{Name: "provider", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "authType", System: true, Required: true})
	col.Fields.Add(&core.JSONField{Name: "credentials", System: true, Required: true, Hidden: true})
	col.Fields.Add(&core.JSONField{Name: "scopes", System: true})
	col.Fields.Add(&core.JSONField{Name: "rateLimits", System: true})
	col.Fields.Add(&core.BoolField{Name: "active", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	col.AddIndex("idx_connectors_provider_active", false, "`provider`, `active`", "")
	return txApp.Save(col)
}

func createAutomationEventsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationEvents); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameAutomationEvents)
	col.System = true
	col.Fields.Add(&core.TextField{Name: "name", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "source", System: true})
	col.Fields.Add(&core.TextField{Name: "subject", System: true})
	col.Fields.Add(&core.JSONField{Name: "payload", System: true})
	col.Fields.Add(&core.DateField{Name: "occurred", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "correlationId", System: true})
	col.Fields.Add(&core.TextField{Name: "causationId", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.AddIndex("idx_automationEvents_name_occurred", false, "`name`, `occurred`", "")
	col.AddIndex("idx_automationEvents_correlation", false, "`correlationId`", "")
	return txApp.Save(col)
}

func createWorkflowVersionsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameWorkflowVersions); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameWorkflowVersions)
	col.System = true
	col.Fields.Add(&core.TextField{Name: "automationRef", System: true, Required: true})
	col.Fields.Add(&core.NumberField{Name: "version", System: true, Required: true, OnlyInt: true})
	col.Fields.Add(&core.TextField{Name: "status", System: true, Required: true})
	col.Fields.Add(&core.JSONField{Name: "snapshot", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "notes", System: true})
	col.Fields.Add(&core.TextField{Name: "createdBy", System: true})
	col.Fields.Add(&core.TextField{Name: "publishedBy", System: true})
	col.Fields.Add(&core.DateField{Name: "publishedAt", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	col.AddIndex("idx_workflowVersions_automation_version", true, "`automationRef`, `version`", "")
	col.AddIndex("idx_workflowVersions_automation_status", false, "`automationRef`, `status`", "")
	return txApp.Save(col)
}

func addAutomationVersionFields(txApp core.App) error {
	col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
	if err != nil {
		return nil
	}
	if col.Fields.GetByName("workflowVersionRef") == nil {
		col.Fields.Add(&core.TextField{Name: "workflowVersionRef", System: true})
	}
	if col.Fields.GetByName("workflowVersionSnapshot") == nil {
		col.Fields.Add(&core.JSONField{Name: "workflowVersionSnapshot", System: true})
	}
	return txApp.Save(col)
}

func addCapabilityConnectorFields(txApp core.App) error {
	col, err := txApp.FindCollectionByNameOrId(core.CollectionNameCapabilities)
	if err != nil {
		return nil
	}
	if col.Fields.GetByName("connectorRef") == nil {
		col.Fields.Add(&core.TextField{Name: "connectorRef", System: true})
	}
	if col.Fields.GetByName("requiredScopes") == nil {
		col.Fields.Add(&core.JSONField{Name: "requiredScopes", System: true})
	}
	return txApp.Save(col)
}
