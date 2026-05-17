package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type IngestResult struct {
	Documents int `json:"documents"`
	Chunks    int `json:"chunks"`
}

type Job struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Tenant    string       `json:"tenant,omitempty"`
	Status    Status       `json:"status"`
	Result    IngestResult `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Manager struct {
	mu    sync.RWMutex
	jobs  map[string]Job
	store *Store
}

func NewManager(store *Store) *Manager {
	return &Manager{jobs: map[string]Job{}, store: store}
}

func (m *Manager) Start(ctx context.Context, jobType string, tenant string, run func(context.Context) (IngestResult, error)) Job {
	job := Job{
		ID:        randomID(),
		Type:      jobType,
		Tenant:    tenant,
		Status:    StatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	m.save(job)

	go func() {
		job.Status = StatusRunning
		job.UpdatedAt = time.Now().UTC()
		m.save(job)

		result, err := run(ctx)
		job.UpdatedAt = time.Now().UTC()
		if err != nil {
			job.Status = StatusFailed
			job.Error = err.Error()
			m.save(job)
			return
		}
		job.Status = StatusCompleted
		job.Result = result
		m.save(job)
	}()

	return job
}

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if ok {
		return job, true
	}
	if m.store != nil && m.store.Enabled() {
		job, ok, err := m.store.Get(context.Background(), id)
		if err == nil && ok {
			m.save(job)
			return job, true
		}
	}
	return Job{}, false
}

func (m *Manager) ListRecent(ctx context.Context, tenant string, limit int) ([]Job, error) {
	if m.store != nil && m.store.Enabled() {
		return m.store.ListRecent(ctx, tenant, limit)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		if tenant != "" && job.Tenant != tenant {
			continue
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (m *Manager) save(job Job) {
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	if m.store != nil && m.store.Enabled() {
		_ = m.store.Save(context.Background(), job)
	}
}

func randomID() string {
	var buf [12]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
