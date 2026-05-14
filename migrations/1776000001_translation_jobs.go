package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		return createTranslationJobsCollection(txApp)
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_translationJobs}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] = {:name}").
			Bind(map[string]any{"name": core.CollectionNameTranslationJobs}).
			Execute()

		return err
	})
}

func createTranslationJobsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameTranslationJobs); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameTranslationJobs)
	col.System = true

	col.Fields.Add(&core.TextField{Name: "collectionRef", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "sourceRecordId", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "targetRecordId", System: true})
	col.Fields.Add(&core.TextField{Name: "sourceLocale", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "targetLocale", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "provider", System: true})
	col.Fields.Add(&core.TextField{Name: "model", System: true})
	col.Fields.Add(&core.TextField{Name: "status", System: true, Required: true})
	col.Fields.Add(&core.TextField{Name: "error", System: true})
	col.Fields.Add(&core.JSONField{Name: "result", System: true})
	col.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})

	col.AddIndex("idx_translationJobs_status_updated", false, "`status`, `updated`", "")
	col.AddIndex("idx_translationJobs_collection_source", false, "`collectionRef`, `sourceRecordId`", "")

	return txApp.Save(col)
}
