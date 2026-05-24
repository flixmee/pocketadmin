package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if err := createAutomationsCollection(txApp); err != nil {
			return err
		}

		return createAutomationRunsCollection(txApp)
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_automationRuns}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_automations}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] IN ({:runName}, {:automationName})").
			Bind(map[string]any{
				"runName":        core.CollectionNameAutomationRuns,
				"automationName": core.CollectionNameAutomations,
			}).
			Execute()
		if err != nil {
			return err
		}

		return nil
	})
}

func createAutomationsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomations); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameAutomations)
	col.System = true

	col.Fields.Add(&core.TextField{
		Name:     "name",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "tag",
		System: true,
	})
	col.Fields.Add(&core.BoolField{
		Name:   "active",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "triggerType",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "collectionRef",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "cronExpr",
		System: true,
	})
	col.Fields.Add(&core.JSONField{
		Name:     "steps",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "notes",
		System: true,
	})
	col.Fields.Add(&core.DateField{
		Name:   "lastRunAt",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "lastRunStatus",
		System: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "created",
		System:   true,
		OnCreate: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "updated",
		System:   true,
		OnCreate: true,
		OnUpdate: true,
	})

	col.AddIndex("idx_automations_active_trigger", false, "active, triggerType", "")
	col.AddIndex("idx_automations_active_collection_trigger", false, "active, collectionRef, triggerType", "")

	return txApp.Save(col)
}

func createAutomationRunsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameAutomationRuns)
	col.System = true

	col.Fields.Add(&core.TextField{
		Name:     "automationRef",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "triggerType",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "status",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.JSONField{
		Name:   "input",
		System: true,
	})
	col.Fields.Add(&core.JSONField{
		Name:   "stepResults",
		System: true,
	})
	col.Fields.Add(&core.TextField{
		Name:   "error",
		System: true,
	})
	col.Fields.Add(&core.NumberField{
		Name:    "errorStepIndex",
		System:  true,
		OnlyInt: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "started",
		System:   true,
		OnCreate: true,
	})
	col.Fields.Add(&core.DateField{
		Name:   "finished",
		System: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "created",
		System:   true,
		OnCreate: true,
	})
	col.Fields.Add(&core.AutodateField{
		Name:     "updated",
		System:   true,
		OnCreate: true,
		OnUpdate: true,
	})

	col.AddIndex("idx_automationRuns_automation_started", false, "automationRef, started", "")
	col.AddIndex("idx_automationRuns_status_started", false, "status, started", "")

	return txApp.Save(col)
}
