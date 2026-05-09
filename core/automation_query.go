package core

import "github.com/pocketbase/dbx"

// FindAutomationById returns a single Automation model by its id.
func (app *BaseApp) FindAutomationById(id string) (*Automation, error) {
	result := &Automation{}

	err := app.RecordQuery(CollectionNameAutomations).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindAutomationRunById returns a single AutomationRun model by its id.
func (app *BaseApp) FindAutomationRunById(id string) (*AutomationRun, error) {
	result := &AutomationRun{}

	err := app.RecordQuery(CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindAllActiveAutomations returns all active Automation models.
func (app *BaseApp) FindAllActiveAutomations() ([]*Automation, error) {
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

// FindAllAutomationRunsByAutomation returns all AutomationRun models linked to the provided automation.
func (app *BaseApp) FindAllAutomationRunsByAutomation(automation *Automation) ([]*AutomationRun, error) {
	result := []*AutomationRun{}

	err := app.RecordQuery(CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"automationRef": automation.Id}).
		OrderBy("started DESC").
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FindAutomationRunsByAutomation returns ordered automation runs with optional limit/offset.
func (app *BaseApp) FindAutomationRunsByAutomation(automation *Automation, limit int, offset int) ([]*AutomationRun, error) {
	result := []*AutomationRun{}

	query := app.RecordQuery(CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"automationRef": automation.Id}).
		OrderBy("started DESC")

	if limit > 0 {
		query.Limit(int64(limit))
	}
	if offset > 0 {
		query.Offset(int64(offset))
	}

	err := query.All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
