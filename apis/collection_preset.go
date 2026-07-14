package apis

import (
	"errors"
	"net/http"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	collectionpresets "github.com/pocketbase/pocketbase/core/presets"
)

func collectionPresetsList(e *core.RequestEvent) error {
	presets, err := collectionpresets.List()
	if err != nil {
		return e.InternalServerError("Failed to load collection presets.", err)
	}

	return e.JSON(http.StatusOK, presets)
}

func collectionPresetPreview(e *core.RequestEvent) error {
	form, err := bindCollectionPresetForm(e)
	if err != nil {
		return err
	}

	preview, err := collectionpresets.BuildPreview(e.App, e.Request.PathValue("preset"), form.Prefix)
	if err != nil {
		return collectionPresetError(e, err)
	}

	return e.JSON(http.StatusOK, preview)
}

func collectionPresetImport(e *core.RequestEvent) error {
	form, err := bindCollectionPresetForm(e)
	if err != nil {
		return err
	}

	preview, err := collectionpresets.BuildPreview(e.App, e.Request.PathValue("preset"), form.Prefix)
	if err != nil {
		return collectionPresetError(e, err)
	}
	if !preview.CanImport {
		messages := make([]string, 0, len(preview.Conflicts))
		for _, conflict := range preview.Conflicts {
			messages = append(messages, conflict.Message)
		}
		return NewApiError(http.StatusConflict, "Collection preset has conflicts.", validation.Errors{
			"conflicts": validation.NewError("validation_collection_preset_conflict", strings.Join(messages, " ")),
		})
	}

	event := new(core.CollectionsImportRequestEvent)
	event.RequestEvent = e
	event.CollectionsData = preview.Collections
	event.DeleteMissing = false
	return triggerCollectionsImport(event)
}

type collectionPresetForm struct {
	Prefix string `form:"prefix" json:"prefix"`
}

func bindCollectionPresetForm(e *core.RequestEvent) (*collectionPresetForm, error) {
	form := new(collectionPresetForm)
	if err := e.BindBody(form); err != nil {
		return nil, firstApiError(err, e.BadRequestError("Failed to load collection preset options.", err))
	}
	return form, nil
}

func collectionPresetError(e *core.RequestEvent, err error) error {
	if errors.Is(err, collectionpresets.ErrNotFound) {
		return e.NotFoundError("Collection preset not found.", nil)
	}

	return e.BadRequestError("Failed to prepare collection preset.", validation.Errors{
		"preset": validation.NewError("validation_collection_preset", err.Error()),
	})
}
