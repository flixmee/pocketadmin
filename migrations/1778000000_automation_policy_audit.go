package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
		if err != nil {
			return nil
		}

		if col.Fields.GetByName("parentRunId") == nil {
			col.Fields.Add(&core.TextField{Name: "parentRunId", System: true})
		}
		if col.Fields.GetByName("depth") == nil {
			col.Fields.Add(&core.NumberField{Name: "depth", System: true, OnlyInt: true})
		}
		if col.Fields.GetByName("dedupeKey") == nil {
			col.Fields.Add(&core.TextField{Name: "dedupeKey", System: true})
		}
		if col.Fields.GetByName("policyDecision") == nil {
			col.Fields.Add(&core.JSONField{Name: "policyDecision", System: true})
		}

		col.AddIndex("idx_automationRuns_parent", false, "`parentRunId`", "")
		col.AddIndex("idx_automationRuns_dedupe_created", false, "`automationRef`, `dedupeKey`, `created`", "`dedupeKey` != ''")

		return txApp.Save(col)
	}, func(txApp core.App) error {
		col, err := txApp.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
		if err != nil {
			return nil
		}

		col.RemoveIndex("idx_automationRuns_parent")
		col.RemoveIndex("idx_automationRuns_dedupe_created")
		col.Fields.RemoveByName("parentRunId")
		col.Fields.RemoveByName("depth")
		col.Fields.RemoveByName("dedupeKey")
		col.Fields.RemoveByName("policyDecision")

		return txApp.Save(col)
	})
}
