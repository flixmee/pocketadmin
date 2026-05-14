package core

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
)

// I18nMigrationOptions defines the options for migrating an existing collection
// to the separate-record i18n model.
type I18nMigrationOptions struct {
	CollectionNameOrId string
	DefaultLocale      string
	LocalizedFields    []string
	DryRun             bool
}

// I18nMigrationReport describes what an i18n migration would do or has done.
type I18nMigrationReport struct {
	CollectionId    string   `json:"collectionId"`
	CollectionName  string   `json:"collectionName"`
	DefaultLocale   string   `json:"defaultLocale"`
	LocalizedFields []string `json:"localizedFields"`
	RecordsTotal    int      `json:"recordsTotal"`
	GroupsToCreate  int      `json:"groupsToCreate"`
	Applied         bool     `json:"applied"`
}

// MigrateCollectionI18n enables i18n on a collection and backfills existing
// rows into one source translation group per record.
func (app *BaseApp) MigrateCollectionI18n(options I18nMigrationOptions) (*I18nMigrationReport, error) {
	nameOrId := strings.TrimSpace(options.CollectionNameOrId)
	if nameOrId == "" {
		return nil, fmt.Errorf("missing collection name or id")
	}

	collection, err := app.FindCollectionByNameOrId(nameOrId)
	if err != nil {
		return nil, err
	}
	if !collection.IsBase() {
		return nil, fmt.Errorf("collection %q doesn't support i18n migration", collection.Name)
	}

	defaultLocale := normalizeLocaleCode(options.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = collection.I18nDefaultLocale(app)
	}
	locale, err := app.FindLocaleByCode(defaultLocale)
	if err != nil || !locale.Enabled() {
		return nil, fmt.Errorf("missing or disabled default locale %q", defaultLocale)
	}

	localizedFields := normalizeI18nMigrationFields(collection, options.LocalizedFields)
	if len(localizedFields) == 0 {
		return nil, fmt.Errorf("missing localized fields")
	}

	var recordsTotal int
	if err := app.RecordQuery(collection).Select("count(*)").Row(&recordsTotal); err != nil {
		return nil, err
	}

	groupsToCreate, err := countI18nRowsMissingGroup(app, collection)
	if err != nil {
		return nil, err
	}

	report := &I18nMigrationReport{
		CollectionId:    collection.Id,
		CollectionName:  collection.Name,
		DefaultLocale:   defaultLocale,
		LocalizedFields: localizedFields,
		RecordsTotal:    recordsTotal,
		GroupsToCreate:  groupsToCreate,
		Applied:         !options.DryRun,
	}

	if options.DryRun {
		return report, nil
	}

	collection.I18n.Enabled = true
	collection.I18n.DefaultLocale = defaultLocale
	collection.I18n.LocalizedFields = localizedFields

	if err := app.Save(collection); err != nil {
		return nil, err
	}

	return report, nil
}

func normalizeI18nMigrationFields(collection *Collection, fields []string) []string {
	seen := map[string]struct{}{}
	result := []string{}

	if len(fields) == 0 {
		for _, field := range collection.Fields {
			if field.GetSystem() || field.GetName() == FieldNameId {
				continue
			}
			result = append(result, field.GetName())
		}
		return result
	}

	for _, fieldName := range fields {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			continue
		}
		if _, ok := seen[fieldName]; ok {
			continue
		}
		if collection.Fields.GetByName(fieldName) == nil {
			continue
		}
		seen[fieldName] = struct{}{}
		result = append(result, fieldName)
	}

	return result
}

func countI18nRowsMissingGroup(app App, collection *Collection) (int, error) {
	if collection == nil || !collection.I18nEnabled() || collection.Fields.GetByName(FieldNameI18nGroupId) == nil {
		var total int
		err := app.RecordQuery(collection).Select("count(*)").Row(&total)
		return total, err
	}

	var total int
	err := app.RecordQuery(collection).
		Select("count(*)").
		AndWhere(dbxEmptyI18nGroupExp()).
		Row(&total)
	return total, err
}

func dbxEmptyI18nGroupExp() dbx.Expression {
	return dbx.NewExp("COALESCE([[" + FieldNameI18nGroupId + "]], '') = '' OR COALESCE([[" + FieldNameLocale + "]], '') = ''")
}
