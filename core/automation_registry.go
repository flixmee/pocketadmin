package core

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/routine"
)

const (
	StoreKeyAutomationRegistry = "pbAppAutomationRegistry"
	automationCronJobPrefix    = "__pbAutomation__"
)

type automationRegistry struct {
	ByID           map[string]*Automation
	ByTrigger      map[string][]*Automation
	ByTriggerScope map[string]map[string][]*Automation
}

func getAutomationRegistry(app App) (*automationRegistry, error) {
	if registry, ok := app.Store().Get(StoreKeyAutomationRegistry).(*automationRegistry); ok && registry != nil {
		return registry, nil
	}

	return refreshAutomationRegistry(app)
}

func refreshAutomationRegistry(app App) (*automationRegistry, error) {
	automations, err := findAllActiveAutomations(app)
	if err != nil {
		return nil, err
	}

	registry := buildAutomationRegistry(automations)

	if err := syncAutomationCronJobs(app, registry); err != nil {
		return nil, err
	}

	app.Store().Set(StoreKeyAutomationRegistry, registry)

	return registry, nil
}

func buildAutomationRegistry(automations []*Automation) *automationRegistry {
	registry := &automationRegistry{
		ByID:           make(map[string]*Automation, len(automations)),
		ByTrigger:      map[string][]*Automation{},
		ByTriggerScope: map[string]map[string][]*Automation{},
	}

	for _, automation := range automations {
		if automation == nil || !automation.Active() {
			continue
		}

		registry.ByID[automation.Id] = automation
		registry.ByTrigger[automation.TriggerType()] = append(registry.ByTrigger[automation.TriggerType()], automation)

		if isRecordAutomationTrigger(automation.TriggerType()) {
			collectionId := automation.CollectionRef()
			if collectionId == "" {
				continue
			}

			if registry.ByTriggerScope[automation.TriggerType()] == nil {
				registry.ByTriggerScope[automation.TriggerType()] = map[string][]*Automation{}
			}

			registry.ByTriggerScope[automation.TriggerType()][collectionId] = append(
				registry.ByTriggerScope[automation.TriggerType()][collectionId],
				automation,
			)
		}
	}

	return registry
}

func syncAutomationCronJobs(app App, registry *automationRegistry) error {
	for _, job := range app.Cron().Jobs() {
		if strings.HasPrefix(job.Id(), automationCronJobPrefix) {
			app.Cron().Remove(job.Id())
		}
	}

	for _, automation := range registry.ByTrigger[AutomationTriggerScheduleCron] {
		if automation == nil || !automation.Active() || strings.TrimSpace(automation.CronExpr()) == "" {
			continue
		}

		automationID := automation.Id
		err := app.Cron().Add(automationCronJobID(automationID), automation.CronExpr(), func() {
			routine.FireAndForget(func() {
				if err := runAutomationByID(app, automationID, automationTriggerPayload{
					TriggerType: AutomationTriggerScheduleCron,
				}); err != nil {
					app.Logger().Warn(
						"Failed to execute scheduled automation",
						"automationId", automationID,
						"error", err,
					)
				}
			})
		})
		if err != nil {
			return fmt.Errorf("failed to register automation cron job %q: %w", automationID, err)
		}
	}

	return nil
}

func automationCronJobID(automationID string) string {
	return automationCronJobPrefix + automationID
}

func findAllActiveAutomations(app App) ([]*Automation, error) {
	result := []*Automation{}

	err := app.RecordQuery(CollectionNameAutomations).
		AndWhere(dbx.HashExp{"active": true}).
		OrderBy("created ASC").
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
