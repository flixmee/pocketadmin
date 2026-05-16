package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if err := createWorkflowStateCollection(txApp); err != nil {
			return err
		}

		return createApprovalsCollection(txApp)
	}, func(txApp core.App) error {
		if _, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_approvals}}").Execute(); err != nil {
			return err
		}
		if _, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_workflowState}}").Execute(); err != nil {
			return err
		}

		_, err := txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] IN ({:stateName}, {:approvalName})").
			Bind(map[string]any{
				"stateName":    core.CollectionNameWorkflowState,
				"approvalName": core.CollectionNameApprovals,
			}).
			Execute()
		return err
	})
}

func createWorkflowStateCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameWorkflowState); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameWorkflowState)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "automationRef", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "runRef", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "status", System: true, Required: true})
	col.Fields.Add(&core.NumberField{Name: "currentStepIndex", System: true, OnlyInt: true})
	col.Fields.Add(&core.JSONField{Name: "context", System: true})
	col.Fields.Add(&core.JSONField{Name: "checkpoints", System: true})
	col.Fields.Add(&core.TextField{Name: "resumeToken", System: true})
	col.Fields.Add(&core.JSONField{Name: "waitingFor", System: true})
	col.Fields.Add(&core.DateField{Name: "expires", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})

	col.AddIndex("idx_workflowState_run", false, "`runRef`", "")
	col.AddIndex("idx_workflowState_status_expires", false, "`status`, `expires`", "")
	col.AddIndex("idx_workflowState_resumeToken", true, "`resumeToken`", "`resumeToken` != ''")

	return txApp.Save(col)
}

func createApprovalsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameApprovals); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameApprovals)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "workflowStateRef", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "automationRef", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "runRef", System: true, Required: true})
	col.Fields.Add(&core.NumberField{Name: "stepIndex", System: true, OnlyInt: true})
	col.Fields.Add(&core.TextField{Name: "assignee", System: true})
	col.Fields.Add(&core.TextField{Name: "role", System: true})
	col.Fields.Add(&core.TextField{Name: "status", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "decision", System: true})
	col.Fields.Add(&core.TextField{Name: "comment", System: true})
	col.Fields.Add(&core.DateField{Name: "resolved", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})

	col.AddIndex("idx_approvals_state", false, "`workflowStateRef`", "")
	col.AddIndex("idx_approvals_status_created", false, "`status`, `created`", "")
	col.AddIndex("idx_approvals_run_step", false, "`runRef`, `stepIndex`", "")

	return txApp.Save(col)
}
