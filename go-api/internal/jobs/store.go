package jobs

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
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
CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	tenant TEXT,
	status TEXT NOT NULL,
	result JSONB NOT NULL DEFAULT '{}'::jsonb,
	error TEXT,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_tenant_created_at ON jobs(tenant, created_at DESC);
`)
	return err
}

func (s *Store) Save(ctx context.Context, job Job) error {
	if !s.Enabled() {
		return nil
	}
	resultJSON, err := json.Marshal(job.Result)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
INSERT INTO jobs (id, type, tenant, status, result, error, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
	type = EXCLUDED.type,
	tenant = EXCLUDED.tenant,
	status = EXCLUDED.status,
	result = EXCLUDED.result,
	error = EXCLUDED.error,
	created_at = EXCLUDED.created_at,
	updated_at = EXCLUDED.updated_at
`, job.ID, job.Type, job.Tenant, string(job.Status), resultJSON, job.Error, job.CreatedAt, job.UpdatedAt)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (Job, bool, error) {
	if !s.Enabled() {
		return Job{}, false, nil
	}
	row := s.pool.QueryRow(ctx, `
SELECT id, type, tenant, status, result, error, created_at, updated_at
FROM jobs
WHERE id = $1
`, id)

	var job Job
	var resultJSON []byte
	var status string
	err := row.Scan(&job.ID, &job.Type, &job.Tenant, &status, &resultJSON, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Job{}, false, nil
		}
		return Job{}, false, err
	}
	job.Status = Status(status)
	if len(resultJSON) > 0 {
		if err := json.Unmarshal(resultJSON, &job.Result); err != nil {
			return Job{}, false, err
		}
	}
	return job, true, nil
}

func (s *Store) ListRecent(ctx context.Context, tenant string, limit int) ([]Job, error) {
	if !s.Enabled() {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `
SELECT id, type, tenant, status, result, error, created_at, updated_at
FROM jobs
`
	args := []any{}
	if tenant != "" {
		query += `WHERE tenant = $1 `
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

	var jobs []Job
	for rows.Next() {
		var job Job
		var resultJSON []byte
		var status string
		if err := rows.Scan(&job.ID, &job.Type, &job.Tenant, &status, &resultJSON, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		job.Status = Status(status)
		if len(resultJSON) > 0 {
			if err := json.Unmarshal(resultJSON, &job.Result); err != nil {
				return nil, err
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
