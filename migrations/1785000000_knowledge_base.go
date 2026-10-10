package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		if err := createKBDocumentsCollection(txApp); err != nil {
			return err
		}
		if err := createKBChunksCollection(txApp); err != nil {
			return err
		}
		if err := createKBChatSessionsCollection(txApp); err != nil {
			return err
		}
		if err := createKBChatMessagesCollection(txApp); err != nil {
			return err
		}
		return createKBFullTextIndex(txApp)
	}, func(txApp core.App) error {
		if _, err := txApp.DB().NewQuery(`
			DROP TRIGGER IF EXISTS [[kb_chunks_fts_insert]];
			DROP TRIGGER IF EXISTS [[kb_chunks_fts_update]];
			DROP TRIGGER IF EXISTS [[kb_chunks_fts_delete]];
			DROP TABLE IF EXISTS [[kb_chunks_fts]];
		`).Execute(); err != nil {
			return err
		}

		for _, name := range []string{
			core.CollectionNameKBChatMessages,
			core.CollectionNameKBChatSessions,
			core.CollectionNameKBChunks,
			core.CollectionNameKBDocuments,
		} {
			collection, err := txApp.FindCollectionByNameOrId(name)
			if err == nil {
				if err := txApp.Delete(collection); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func createKBDocumentsCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments); err == nil {
		return nil
	}

	collection := core.NewBaseCollection(core.CollectionNameKBDocuments)
	collection.System = true
	collection.Fields.Add(&core.TextField{Name: "title", System: true, Required: true, Max: 255, Presentable: true})
	collection.Fields.Add(&core.FileField{
		Name:      "file",
		System:    true,
		Required:  true,
		Protected: true,
		MaxSize:   core.MaxKnowledgeDocumentBytes,
		MimeTypes: []string{
			"text/plain",
			"text/markdown",
			"text/html",
			"application/xhtml+xml",
			"application/pdf",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
	})
	collection.Fields.Add(&core.TextField{Name: "mime_type", System: true, Max: 255})
	collection.Fields.Add(&core.SelectField{
		Name:     "status",
		System:   true,
		Required: true,
		Values: []string{
			core.KBDocumentStatusPending,
			core.KBDocumentStatusProcessing,
			core.KBDocumentStatusIndexed,
			core.KBDocumentStatusFailed,
		},
	})
	collection.Fields.Add(&core.TextField{Name: "owner", System: true, Required: true})
	collection.Fields.Add(&core.TextField{Name: "owner_collection", System: true, Required: true})
	collection.Fields.Add(&core.JSONField{Name: "metadata", System: true})
	collection.Fields.Add(&core.TextField{Name: "error", System: true, Max: 5000})
	collection.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	collection.AddIndex("idx_kb_documents_owner_created", false, "owner_collection, owner, created", "")
	collection.AddIndex("idx_kb_documents_status_updated", false, "status, updated", "")
	return app.Save(collection)
}

func createKBChunksCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameKBChunks); err == nil {
		return nil
	}
	documents, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection(core.CollectionNameKBChunks)
	collection.System = true
	collection.Fields.Add(&core.RelationField{
		Name:          "document_id",
		System:        true,
		Required:      true,
		CollectionId:  documents.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "content", System: true, Required: true, Max: 20000})
	collection.Fields.Add(&core.NumberField{Name: "chunk_index", System: true, OnlyInt: true})
	collection.Fields.Add(&core.JSONField{Name: "embedding", System: true, Hidden: true, Required: true, MaxSize: 4 << 20})
	collection.Fields.Add(&core.NumberField{Name: "token_count", System: true, OnlyInt: true})
	collection.Fields.Add(&core.JSONField{Name: "metadata", System: true})
	collection.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	collection.AddIndex("idx_kb_chunks_document_index", true, "document_id, chunk_index", "")
	return app.Save(collection)
}

func createKBChatSessionsCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameKBChatSessions); err == nil {
		return nil
	}

	collection := core.NewBaseCollection(core.CollectionNameKBChatSessions)
	collection.System = true
	collection.Fields.Add(&core.TextField{Name: "user", System: true, Required: true})
	collection.Fields.Add(&core.TextField{Name: "user_collection", System: true, Required: true})
	collection.Fields.Add(&core.TextField{Name: "title", System: true, Required: true, Max: 255, Presentable: true})
	collection.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	collection.AddIndex("idx_kb_chat_sessions_user_updated", false, "user_collection, user, updated", "")
	return app.Save(collection)
}

func createKBChatMessagesCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameKBChatMessages); err == nil {
		return nil
	}
	sessions, err := app.FindCollectionByNameOrId(core.CollectionNameKBChatSessions)
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection(core.CollectionNameKBChatMessages)
	collection.System = true
	collection.Fields.Add(&core.RelationField{
		Name:          "session_id",
		System:        true,
		Required:      true,
		CollectionId:  sessions.Id,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.SelectField{
		Name:     "role",
		System:   true,
		Required: true,
		Values:   []string{core.KBChatRoleUser, core.KBChatRoleAssistant},
	})
	collection.Fields.Add(&core.TextField{Name: "content", System: true, Required: true, Max: 50000})
	collection.Fields.Add(&core.JSONField{Name: "retrieved_chunk_ids", System: true})
	collection.Fields.Add(&core.TextField{Name: "model", System: true, Max: 255})
	collection.Fields.Add(&core.JSONField{Name: "token_usage", System: true})
	collection.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", System: true, OnCreate: true, OnUpdate: true})
	collection.AddIndex("idx_kb_chat_messages_session_created", false, "session_id, created", "")
	return app.Save(collection)
}

func createKBFullTextIndex(app core.App) error {
	_, err := app.DB().NewQuery(`
		CREATE VIRTUAL TABLE IF NOT EXISTS [[kb_chunks_fts]] USING fts5(
			[[content]],
			[[chunk_id]] UNINDEXED,
			tokenize = 'unicode61 remove_diacritics 2'
		);

		CREATE TRIGGER IF NOT EXISTS [[kb_chunks_fts_insert]] AFTER INSERT ON [[kb_chunks]] BEGIN
			INSERT INTO [[kb_chunks_fts]] ([[rowid]], [[content]], [[chunk_id]])
			VALUES (new.[[_rowid_]], new.[[content]], new.[[id]]);
		END;

		CREATE TRIGGER IF NOT EXISTS [[kb_chunks_fts_update]] AFTER UPDATE OF [[content]] ON [[kb_chunks]] BEGIN
			DELETE FROM [[kb_chunks_fts]] WHERE [[rowid]] = old.[[_rowid_]];
			INSERT INTO [[kb_chunks_fts]] ([[rowid]], [[content]], [[chunk_id]])
			VALUES (new.[[_rowid_]], new.[[content]], new.[[id]]);
		END;

		CREATE TRIGGER IF NOT EXISTS [[kb_chunks_fts_delete]] AFTER DELETE ON [[kb_chunks]] BEGIN
			DELETE FROM [[kb_chunks_fts]] WHERE [[rowid]] = old.[[_rowid_]];
		END;

		INSERT INTO [[kb_chunks_fts]] ([[rowid]], [[content]], [[chunk_id]])
		SELECT c.[[_rowid_]], c.[[content]], c.[[id]]
		FROM [[kb_chunks]] c
		WHERE NOT EXISTS (
			SELECT 1 FROM [[kb_chunks_fts]] f WHERE f.[[rowid]] = c.[[_rowid_]]
		);
	`).Execute()
	if err != nil {
		return fmt.Errorf("failed to create knowledge FTS index: %w", err)
	}
	return nil
}
