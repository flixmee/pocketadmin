package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if err := createLocalesCollection(txApp); err != nil {
			return err
		}
		if err := createI18nGroupsCollection(txApp); err != nil {
			return err
		}

		return txApp.EnsureDefaultLocale()
	}, func(txApp core.App) error {
		_, err := txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_i18nGroups}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().NewQuery("DROP TABLE IF EXISTS {{_locales}}").Execute()
		if err != nil {
			return err
		}

		_, err = txApp.DB().
			NewQuery("DELETE FROM {{_collections}} WHERE [[name]] IN ({:locales}, {:groups})").
			Bind(map[string]any{
				"locales": core.CollectionNameLocales,
				"groups":  core.CollectionNameI18nGroups,
			}).
			Execute()

		return err
	})
}

func createLocalesCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameLocales); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameLocales)
	col.System = true

	col.Fields.Add(&core.TextField{
		Name:     "code",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "label",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.BoolField{
		Name:   "enabled",
		System: true,
	})
	col.Fields.Add(&core.BoolField{
		Name:   "is_default",
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

	col.AddIndex("idx_locales_code", true, "`code`", "")
	col.AddIndex("idx_locales_default", false, "`is_default`", "")

	return txApp.Save(col)
}

func createI18nGroupsCollection(txApp core.App) error {
	if _, err := txApp.FindCollectionByNameOrId(core.CollectionNameI18nGroups); err == nil {
		return nil
	}

	col := core.NewBaseCollection(core.CollectionNameI18nGroups)
	col.System = true

	col.Fields.Add(&core.TextField{
		Name:     "collectionRef",
		System:   true,
		Required: true,
	})
	col.Fields.Add(&core.TextField{
		Name:     "defaultLocale",
		System:   true,
		Required: true,
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

	col.AddIndex("idx_i18n_groups_collection", false, "`collectionRef`", "")

	return txApp.Save(col)
}
