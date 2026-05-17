package metadata

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hybrid-rag-engine/go-api/internal/chunking"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return &Store{}, nil
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	store := &Store{pool: pool}
	if err := store.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Enabled() bool {
	return s != nil && s.pool != nil
}

func (s *Store) Close() {
	if s.Enabled() {
		s.pool.Close()
	}
}

func (s *Store) Migrate(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	source TEXT,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chunks (
	id TEXT PRIMARY KEY,
	document_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	source TEXT,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_source ON documents(source);
CREATE INDEX IF NOT EXISTS idx_documents_metadata ON documents USING GIN(metadata);
CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunks_source ON chunks(source);
CREATE INDEX IF NOT EXISTS idx_chunks_metadata ON chunks USING GIN(metadata);
`)
	return err
}

func (s *Store) UpsertRawDocuments(ctx context.Context, docs []chunking.RawDocument) error {
	if !s.Enabled() {
		return nil
	}
	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return err
		}
		_, err = s.pool.Exec(ctx, `
INSERT INTO documents (id, title, content, source, metadata, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
	title = EXCLUDED.title,
	content = EXCLUDED.content,
	source = EXCLUDED.source,
	metadata = EXCLUDED.metadata,
	updated_at = EXCLUDED.updated_at
`, scopedID(doc.Metadata, doc.ID), doc.Title, doc.Content, doc.Source, metadataJSON, time.Now().UTC())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertChunks(ctx context.Context, chunks []retrieval.Document) error {
	if !s.Enabled() {
		return nil
	}
	for _, chunk := range chunks {
		metadataJSON, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}
		documentID := chunk.ID
		if chunk.Metadata != nil && chunk.Metadata["document_id"] != "" {
			documentID = chunk.Metadata["document_id"]
		}
		_, err = s.pool.Exec(ctx, `
INSERT INTO chunks (id, document_id, title, content, source, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
	document_id = EXCLUDED.document_id,
	title = EXCLUDED.title,
	content = EXCLUDED.content,
	source = EXCLUDED.source,
	metadata = EXCLUDED.metadata
`, scopedID(chunk.Metadata, chunk.ID), scopedID(chunk.Metadata, documentID), chunk.Title, chunk.Content, chunk.Source, metadataJSON)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListChunks(ctx context.Context, limit int, tenant string) ([]retrieval.Document, error) {
	if !s.Enabled() {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `
SELECT id, title, content, COALESCE(source, ''), metadata
FROM chunks
`
	args := []any{}
	if tenant != "" {
		query += `WHERE metadata->>'tenant' = $1 `
		args = append(args, tenant)
	}
	query += `ORDER BY created_at DESC `
	args = append(args, limit)
	query += `LIMIT $` + itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []retrieval.Document
	for rows.Next() {
		var doc retrieval.Document
		var metadataJSON []byte
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.Source, &metadataJSON); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
				return nil, err
			}
		}
		doc.ID = unscopedID(doc.Metadata, doc.ID)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func scopedID(metadata map[string]string, id string) string {
	if metadata != nil && metadata["tenant"] != "" {
		return metadata["tenant"] + ":" + id
	}
	return id
}

func unscopedID(metadata map[string]string, id string) string {
	if metadata == nil {
		return id
	}
	tenant := metadata["tenant"]
	prefix := tenant + ":"
	if tenant != "" && strings.HasPrefix(id, prefix) {
		return strings.TrimPrefix(id, prefix)
	}
	return id
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
