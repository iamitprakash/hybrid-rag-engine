package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	mu   sync.RWMutex
	jobs map[string]Job
}

func NewManager() *Manager {
	return &Manager{jobs: map[string]Job{}}
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
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	return job, ok
}

func (m *Manager) save(job Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
}

func randomID() string {
	var buf [12]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
