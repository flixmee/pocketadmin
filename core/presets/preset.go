// Package presets loads and resolves built-in collection presets.
package presets

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

var (
	// ErrNotFound indicates that a requested built-in preset doesn't exist.
	ErrNotFound = errors.New("collection preset not found")

	presetIDRegex      = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	versionRegex       = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)
	collectionNameExpr = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	prefixRegex        = regexp.MustCompile(`^[A-Za-z](?:[A-Za-z0-9_]*[A-Za-z0-9])?$`)
	collectionRefRegex = regexp.MustCompile(`^\$collection\.([A-Za-z_][A-Za-z0-9_]*)$`)
)

//go:embed *.json
var presetFiles embed.FS

// Preset is the persisted definition of a built-in collection preset.
type Preset struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Description string           `json:"description"`
	Collections []map[string]any `json:"collections"`
	SampleData  []map[string]any `json:"sampleData"`
}

// Summary is the public catalog representation of a preset.
type Summary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	Collections     []string `json:"collections"`
	CollectionCount int      `json:"collectionCount"`
}

// Relationship describes a resolved relation field in a preview.
type Relationship struct {
	Collection       string `json:"collection"`
	Field            string `json:"field"`
	TargetCollection string `json:"targetCollection"`
	External         bool   `json:"external"`
}

// Conflict describes an existing collection that prevents a create-only import.
type Conflict struct {
	Collection string `json:"collection"`
	Type       string `json:"type"`
	ExistingID string `json:"existingId"`
	Message    string `json:"message"`
}

// Preview is the fully resolved, side-effect-free import plan.
type Preview struct {
	Preset        Summary          `json:"preset"`
	Prefix        string           `json:"prefix"`
	Collections   []map[string]any `json:"collections"`
	Relationships []Relationship   `json:"relationships"`
	Conflicts     []Conflict       `json:"conflicts"`
	Warnings      []string         `json:"warnings"`
	CanImport     bool             `json:"canImport"`
}

// List returns metadata for all embedded built-in presets.
func List() ([]Summary, error) {
	paths, err := fs.Glob(presetFiles, "*.json")
	if err != nil {
		return nil, fmt.Errorf("list collection presets: %w", err)
	}
	slices.Sort(paths)

	result := make([]Summary, 0, len(paths))
	for _, path := range paths {
		preset, err := loadPath(path)
		if err != nil {
			return nil, err
		}
		result = append(result, summarize(preset))
	}

	return result, nil
}

// Get loads a single embedded preset by ID.
func Get(id string) (*Preset, error) {
	id = strings.TrimSpace(id)
	if !presetIDRegex.MatchString(id) {
		return nil, ErrNotFound
	}

	preset, err := loadPath(id + ".json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	return preset, err
}

// BuildPreview resolves names, IDs, symbolic relations, and conflicts without
// changing application state.
func BuildPreview(app core.App, id, rawPrefix string) (*Preview, error) {
	preset, err := Get(id)
	if err != nil {
		return nil, err
	}

	prefix, err := normalizePrefix(rawPrefix)
	if err != nil {
		return nil, err
	}

	collections, err := cloneCollections(preset.Collections)
	if err != nil {
		return nil, err
	}

	resolvedNames := make(map[string]string, len(collections))
	resolvedIDs := make(map[string]string, len(collections))
	for _, collection := range collections {
		name, _ := collection["name"].(string)
		typ, _ := collection["type"].(string)
		resolvedName := prefixedName(prefix, name)
		if len(resolvedName) > 255 {
			return nil, fmt.Errorf("prefix produces collection name %q longer than 255 characters", resolvedName)
		}

		resolvedNames[name] = resolvedName
		resolvedIDs[name] = core.NewCollection(typ, resolvedName).Id
		collection["name"] = resolvedName
		collection["id"] = resolvedIDs[name]
	}

	preview := &Preview{
		Preset:        summarize(preset),
		Prefix:        prefix,
		Collections:   collections,
		Relationships: []Relationship{},
		Conflicts:     []Conflict{},
		Warnings:      []string{},
	}

	for _, collection := range collections {
		collectionName, _ := collection["name"].(string)
		fields, _ := collection["fields"].([]any)
		for _, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("preset %q collection %q contains an invalid field", preset.ID, collectionName)
			}

			rawRef, exists := field["collectionRef"]
			if !exists {
				continue
			}

			ref, ok := rawRef.(string)
			if !ok {
				return nil, fmt.Errorf("preset %q collection %q has a non-string collectionRef", preset.ID, collectionName)
			}
			matches := collectionRefRegex.FindStringSubmatch(ref)
			if len(matches) != 2 {
				return nil, fmt.Errorf("preset %q contains invalid collection reference %q", preset.ID, ref)
			}

			targetKey := matches[1]
			targetName, internal := resolvedNames[targetKey]
			targetID := resolvedIDs[targetKey]
			if !internal {
				target, findErr := app.FindCachedCollectionByNameOrId(targetKey)
				if findErr != nil {
					if errors.Is(findErr, sql.ErrNoRows) {
						return nil, fmt.Errorf("preset %q references missing collection %q", preset.ID, targetKey)
					}
					return nil, fmt.Errorf("resolve collection reference %q: %w", targetKey, findErr)
				}
				targetName = target.Name
				targetID = target.Id
			}

			delete(field, "collectionRef")
			field["collectionId"] = targetID
			fieldName, _ := field["name"].(string)
			preview.Relationships = append(preview.Relationships, Relationship{
				Collection:       collectionName,
				Field:            fieldName,
				TargetCollection: targetName,
				External:         !internal,
			})
		}
	}

	for _, collection := range collections {
		name, _ := collection["name"].(string)
		id, _ := collection["id"].(string)
		conflicts, err := detectConflicts(app, name, id)
		if err != nil {
			return nil, err
		}
		preview.Conflicts = append(preview.Conflicts, conflicts...)
	}

	preview.CanImport = len(preview.Conflicts) == 0
	return preview, nil
}

func loadPath(path string) (*Preset, error) {
	raw, err := presetFiles.ReadFile(path)
	if err != nil {
		return nil, err
	}

	preset := new(Preset)
	if err := json.Unmarshal(raw, preset); err != nil {
		return nil, fmt.Errorf("decode collection preset %q: %w", path, err)
	}
	if err := validate(preset, strings.TrimSuffix(path, ".json")); err != nil {
		return nil, fmt.Errorf("validate collection preset %q: %w", path, err)
	}

	return preset, nil
}

func validate(preset *Preset, expectedID string) error {
	if preset.ID != expectedID || !presetIDRegex.MatchString(preset.ID) {
		return fmt.Errorf("invalid id %q", preset.ID)
	}
	if strings.TrimSpace(preset.Name) == "" {
		return errors.New("name is required")
	}
	if !versionRegex.MatchString(preset.Version) {
		return fmt.Errorf("unsupported version %q (expected major version 1)", preset.Version)
	}
	if len(preset.Collections) == 0 {
		return errors.New("at least one collection is required")
	}
	if len(preset.SampleData) > 0 {
		return errors.New("sampleData is not supported by the Phase 1 importer")
	}

	names := make(map[string]struct{}, len(preset.Collections))
	for i, collection := range preset.Collections {
		name, _ := collection["name"].(string)
		if !collectionNameExpr.MatchString(name) {
			return fmt.Errorf("collection %d has invalid name %q", i, name)
		}
		if _, exists := names[strings.ToLower(name)]; exists {
			return fmt.Errorf("duplicate collection name %q", name)
		}
		names[strings.ToLower(name)] = struct{}{}

		typ, _ := collection["type"].(string)
		if typ != core.CollectionTypeBase && typ != core.CollectionTypeAuth && typ != core.CollectionTypeView {
			return fmt.Errorf("collection %q has invalid type %q", name, typ)
		}
		if _, hasID := collection["id"]; hasID {
			return fmt.Errorf("collection %q must not define a database-specific id", name)
		}
	}

	return nil
}

func normalizePrefix(raw string) (string, error) {
	prefix := strings.TrimSpace(raw)
	if prefix == "" {
		return "", nil
	}
	if len(prefix) > 100 || !prefixRegex.MatchString(prefix) {
		return "", errors.New("prefix must start with a letter, end with a letter or number, and contain only letters, numbers, or underscores")
	}
	return prefix, nil
}

func prefixedName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "_" + name
}

func summarize(preset *Preset) Summary {
	collections := make([]string, 0, len(preset.Collections))
	for _, collection := range preset.Collections {
		name, _ := collection["name"].(string)
		collections = append(collections, name)
	}
	return Summary{
		ID:              preset.ID,
		Name:            preset.Name,
		Version:         preset.Version,
		Description:     preset.Description,
		Collections:     collections,
		CollectionCount: len(collections),
	}
}

func cloneCollections(collections []map[string]any) ([]map[string]any, error) {
	raw, err := json.Marshal(collections)
	if err != nil {
		return nil, fmt.Errorf("clone collection preset: %w", err)
	}

	cloned := []map[string]any{}
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, fmt.Errorf("clone collection preset: %w", err)
	}
	return cloned, nil
}

func detectConflicts(app core.App, name, id string) ([]Conflict, error) {
	result := []Conflict{}
	seen := map[string]struct{}{}

	for _, check := range []struct {
		value string
		typ   string
	}{
		{value: name, typ: "name"},
		{value: id, typ: "id"},
	} {
		existing, err := app.FindCachedCollectionByNameOrId(check.value)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("detect collection conflict for %q: %w", name, err)
		}
		key := existing.Id
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		message := fmt.Sprintf("Collection name %q already exists.", name)
		if check.typ == "id" && !strings.EqualFold(existing.Name, name) {
			message = fmt.Sprintf("Generated collection id %q is already used by %q.", id, existing.Name)
		}
		result = append(result, Conflict{
			Collection: name,
			Type:       check.typ,
			ExistingID: existing.Id,
			Message:    message,
		})
	}

	return result, nil
}
