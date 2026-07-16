package core

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/list"
)

var _ optionsValidator = (*collectionBaseOptions)(nil)

// collectionBaseOptions defines the options for the "base" type collection.
type collectionBaseOptions struct {
	I18n        I18nCollectionOptions `form:"i18n" json:"i18n"`
	TableFields []string              `form:"tableFields" json:"tableFields"`
}

func (o *collectionBaseOptions) validate(cv *collectionValidator) error {
	return o.I18n.validate(cv)
}

// I18nCollectionOptions defines localization options for base collections.
type I18nCollectionOptions struct {
	Enabled         bool     `form:"enabled" json:"enabled"`
	DefaultLocale   string   `form:"defaultLocale" json:"defaultLocale"`
	LocalizedFields []string `form:"localizedFields" json:"localizedFields"`
}

func (o I18nCollectionOptions) validate(cv *collectionValidator) error {
	if !o.Enabled {
		return nil
	}

	errs := validation.Errors{}

	defaultLocale := normalizeLocaleCode(o.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = cv.app.DefaultLocaleCode()
	}
	if defaultLocale == "" {
		defaultLocale = DefaultLocaleCode
	}
	if err := ValidateLocaleCode(defaultLocale); err != nil {
		errs["defaultLocale"] = err
	} else if _, err := cv.app.FindLocaleByCode(defaultLocale); err != nil {
		errs["defaultLocale"] = validation.NewError("validation_unknown_locale", "Unknown or disabled default locale.")
	}

	fieldNames := cv.new.Fields.FieldNames()
	seen := []string{}
	for i, name := range o.LocalizedFields {
		name = normalizeFieldName(name)
		if name == "" {
			errs[fmt.Sprintf("localizedFields.%d", i)] = validation.NewError("validation_required", "Missing field name.")
			continue
		}
		if list.ExistInSlice(name, seen) {
			errs[fmt.Sprintf("localizedFields.%d", i)] = validation.NewError("validation_duplicated_field_name", "Duplicated localized field name.")
			continue
		}
		if !list.ExistInSlice(name, fieldNames) {
			errs[fmt.Sprintf("localizedFields.%d", i)] = validation.NewError("validation_unknown_field", "Unknown localized field name.")
			continue
		}
		seen = append(seen, name)
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func normalizeFieldName(name string) string {
	return strings.TrimSpace(name)
}
