package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// NewI18nCommand creates and returns a command for i18n utilities.
func NewI18nCommand(app core.App) *cobra.Command {
	command := &cobra.Command{
		Use:   "migrate:i18n COLLECTION [field...]",
		Short: "Enables i18n for an existing collection",
		Example: strings.Join([]string{
			"migrate:i18n posts title content slug",
			"migrate:i18n posts --defaultLocale vi --dryRun",
		}, "\n"),
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(command *cobra.Command, args []string) error {
			defaultLocale, _ := command.Flags().GetString("defaultLocale")
			dryRun, _ := command.Flags().GetBool("dryRun")

			report, err := migrateI18n(app, args[0], args[1:], defaultLocale, dryRun)
			if err != nil {
				return err
			}

			if report.Applied {
				color.Green("Enabled i18n for collection %q.", report.CollectionName)
			} else {
				color.Yellow("Dry run for i18n migration on collection %q.", report.CollectionName)
			}
			fmt.Fprintf(command.OutOrStdout(), "Default locale: %s\n", report.DefaultLocale)
			fmt.Fprintf(command.OutOrStdout(), "Localized fields: %s\n", strings.Join(report.LocalizedFields, ", "))
			fmt.Fprintf(command.OutOrStdout(), "Records: %d\n", report.RecordsTotal)
			fmt.Fprintf(command.OutOrStdout(), "Groups to create: %d\n", report.GroupsToCreate)

			return nil
		},
	}

	command.Flags().String("defaultLocale", "", "default locale code to assign to existing records")
	command.Flags().Bool("dryRun", false, "print the migration report without applying changes")

	return command
}

type i18nMigrator interface {
	MigrateCollectionI18n(options core.I18nMigrationOptions) (*core.I18nMigrationReport, error)
}

func migrateI18n(app core.App, collection string, fields []string, defaultLocale string, dryRun bool) (*core.I18nMigrationReport, error) {
	migrator, ok := app.(i18nMigrator)
	if !ok {
		return nil, fmt.Errorf("app doesn't support i18n migrations")
	}

	return migrator.MigrateCollectionI18n(core.I18nMigrationOptions{
		CollectionNameOrId: collection,
		DefaultLocale:      defaultLocale,
		LocalizedFields:    fields,
		DryRun:             dryRun,
	})
}
