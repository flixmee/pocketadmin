package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const MCPKeysTableName = "_mcpKeys"

// MCP key permissions.
const (
	MCPPermissionCollectionsRead  = "collections.read"
	MCPPermissionCollectionsWrite = "collections.write"
	MCPPermissionRecordsRead      = "records.read"
	MCPPermissionRecordsWrite     = "records.write"
	MCPPermissionMediaRead        = "media.read"
	MCPPermissionMediaWrite       = "media.write"
)

// MCPPermissions lists all supported MCP key permissions.
var MCPPermissions = []string{
	MCPPermissionCollectionsRead,
	MCPPermissionCollectionsWrite,
	MCPPermissionRecordsRead,
	MCPPermissionRecordsWrite,
	MCPPermissionMediaRead,
	MCPPermissionMediaWrite,
}

const (
	mcpKeyTokenPrefix     = "pbmcp_"
	mcpKeyTokenLength     = 40
	mcpKeyDisplayLength   = len(mcpKeyTokenPrefix) + 6
	mcpKeyLastUsedRefresh = time.Minute
)

var _ Model = (*MCPKey)(nil)

// MCPKey defines an API key that grants an MCP client scoped access
// to the app collections and records.
type MCPKey struct {
	BaseModel

	Name        string                  `db:"name" json:"name"`
	KeyHash     string                  `db:"keyHash" json:"-"`
	KeyPrefix   string                  `db:"keyPrefix" json:"keyPrefix"`
	Permissions types.JSONArray[string] `db:"permissions" json:"permissions"`
	Collections types.JSONArray[string] `db:"collections" json:"collections"`
	Active      bool                    `db:"active" json:"active"`
	ExpiresAt   types.DateTime          `db:"expiresAt" json:"expiresAt"`
	LastUsedAt  types.DateTime          `db:"lastUsedAt" json:"lastUsedAt"`
	Created     types.DateTime          `db:"created" json:"created"`
	Updated     types.DateTime          `db:"updated" json:"updated"`
}

func (m *MCPKey) TableName() string {
	return MCPKeysTableName
}

// HasPermission reports whether the key was granted the specified permission.
func (m *MCPKey) HasPermission(permission string) bool {
	return slices.Contains(m.Permissions, permission)
}

// IsExpired reports whether the key has an expiration date in the past.
func (m *MCPKey) IsExpired() bool {
	return !m.ExpiresAt.IsZero() && m.ExpiresAt.Time().Before(time.Now())
}

// CanAccessCollection reports whether the key is allowed to access the
// specified collection.
//
// System collections are never accessible. An empty allowlist means
// that all non-system collections are accessible.
func (m *MCPKey) CanAccessCollection(collection *Collection) bool {
	if collection == nil || collection.System {
		return false
	}

	if len(m.Collections) == 0 {
		return true
	}

	return slices.Contains(m.Collections, collection.Id)
}

// Validate validates the user configurable key fields.
func (m *MCPKey) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&m.Permissions, validation.Each(validation.In(toAnySlice(MCPPermissions)...))),
	)
}

// NewMCPKey initializes a new active MCP key model and returns it
// together with its plain token (the token is not stored and cannot be recovered).
func NewMCPKey() (*MCPKey, string) {
	m := &MCPKey{
		Active:      true,
		Permissions: types.JSONArray[string]{},
		Collections: types.JSONArray[string]{},
	}
	m.Id = security.RandomString(15)

	token := m.RefreshToken()

	return m, token
}

// RefreshToken generates and assigns a new token hash to the key and returns the new plain token.
func (m *MCPKey) RefreshToken() string {
	token := mcpKeyTokenPrefix + security.RandomString(mcpKeyTokenLength)

	m.KeyHash = HashMCPKeyToken(token)
	m.KeyPrefix = token[:mcpKeyDisplayLength]

	return token
}

// HashMCPKeyToken returns the stored hash representation of a plain MCP key token.
func HashMCPKeyToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SaveMCPKey validates and persists the provided MCP key (insert or update).
func SaveMCPKey(app App, m *MCPKey) error {
	m.Name = strings.TrimSpace(m.Name)
	m.Permissions = normalizeMCPStrings(m.Permissions)
	m.Collections = normalizeMCPStrings(m.Collections)

	if err := m.Validate(); err != nil {
		return err
	}

	if m.KeyHash == "" {
		return errors.New("missing MCP key hash")
	}

	now := types.NowDateTime()
	m.Updated = now

	params := dbx.Params{
		"name":        m.Name,
		"keyHash":     m.KeyHash,
		"keyPrefix":   m.KeyPrefix,
		"permissions": m.Permissions,
		"collections": m.Collections,
		"active":      m.Active,
		"expiresAt":   m.ExpiresAt.String(),
		"updated":     m.Updated.String(),
	}

	if m.IsNew() {
		if m.Id == "" {
			m.Id = security.RandomString(15)
		}
		m.Created = now
		params["id"] = m.Id
		params["created"] = m.Created.String()
		params["lastUsedAt"] = m.LastUsedAt.String()

		if _, err := app.NonconcurrentDB().Insert(MCPKeysTableName, params).Execute(); err != nil {
			return err
		}

		m.MarkAsNotNew()

		return nil
	}

	_, err := app.NonconcurrentDB().Update(MCPKeysTableName, params, dbx.HashExp{"id": m.Id}).Execute()

	return err
}

// DeleteMCPKey deletes the MCP key with the specified id.
func DeleteMCPKey(app App, id string) error {
	_, err := app.NonconcurrentDB().Delete(MCPKeysTableName, dbx.HashExp{"id": id}).Execute()
	return err
}

// FindMCPKeyById returns a single MCP key by its id.
func FindMCPKeyById(app App, id string) (*MCPKey, error) {
	m := &MCPKey{}

	err := app.ModelQuery(m).AndWhere(dbx.HashExp{"id": id}).Limit(1).One(m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// FindAllMCPKeys returns all MCP keys ordered by their creation date (newest first).
func FindAllMCPKeys(app App) ([]*MCPKey, error) {
	keys := []*MCPKey{}

	err := app.ModelQuery(&MCPKey{}).OrderBy("created DESC", "id DESC").All(&keys)
	if err != nil {
		return nil, err
	}

	return keys, nil
}

// FindActiveMCPKeyByToken returns the active and non-expired MCP key
// matching the provided plain token.
//
// On success the key lastUsedAt is also refreshed (at most once per minute).
func FindActiveMCPKeyByToken(app App, token string) (*MCPKey, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, mcpKeyTokenPrefix) {
		return nil, errors.New("invalid MCP key")
	}

	m := &MCPKey{}

	err := app.ModelQuery(m).AndWhere(dbx.HashExp{"keyHash": HashMCPKeyToken(token)}).Limit(1).One(m)
	if err != nil {
		return nil, errors.New("invalid MCP key")
	}

	if !m.Active {
		return nil, errors.New("the MCP key is disabled")
	}

	if m.IsExpired() {
		return nil, errors.New("the MCP key has expired")
	}

	if m.LastUsedAt.IsZero() || time.Since(m.LastUsedAt.Time()) > mcpKeyLastUsedRefresh {
		m.LastUsedAt = types.NowDateTime()
		_, err := app.NonconcurrentDB().Update(
			MCPKeysTableName,
			dbx.Params{"lastUsedAt": m.LastUsedAt.String()},
			dbx.HashExp{"id": m.Id},
		).Execute()
		if err != nil {
			app.Logger().Warn("Failed to update MCP key lastUsedAt", "keyId", m.Id, "error", err)
		}
	}

	return m, nil
}

func normalizeMCPStrings(values []string) types.JSONArray[string] {
	result := make(types.JSONArray[string], 0, len(values))

	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !slices.Contains(result, v) {
			result = append(result, v)
		}
	}

	return result
}
