package taskqueue

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Status      Status         `json:"status"`
	Message     string         `json:"message,omitempty"`
	Error       string         `json:"error,omitempty"`
	RequestedBy string         `json:"requested_by,omitempty"`
	Completed   int            `json:"completed"`
	Total       int            `json:"total"`
	Percent     float64        `json:"percent"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Result      any            `json:"result,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
}

type Summary struct {
	Active int `json:"active"`
	Failed int `json:"failed"`
	Total  int `json:"total"`
}

type Manager struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	order    []string
	maxTasks int
	log      *zap.Logger
}

type Handle struct {
	manager *Manager
	taskID  string
}

func New(log *zap.Logger) *Manager {
	if log == nil {
		log = zap.NewNop()
	}
	return &Manager{
		tasks:    map[string]*Task{},
		order:    []string{},
		maxTasks: 200,
		log:      log,
	}
}

func (m *Manager) Start(kind, requestedBy string, metadata map[string]any, run func(context.Context, *Handle) (any, error)) Task {
	now := time.Now().UTC()
	task := &Task{
		ID:          uuid.NewString(),
		Kind:        kind,
		Status:      StatusQueued,
		RequestedBy: requestedBy,
		Metadata:    cloneMap(metadata),
		CreatedAt:   now,
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.order = append(m.order, task.ID)
	m.trimLocked()
	m.mu.Unlock()

	go m.run(task.ID, run)
	return m.GetOrZero(task.ID)
}

func (m *Manager) List(limit int) []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.order) {
		limit = len(m.order)
	}

	result := make([]Task, 0, limit)
	for i := len(m.order) - 1; i >= 0 && len(result) < limit; i-- {
		if task, ok := m.tasks[m.order[i]]; ok {
			result = append(result, cloneTask(task))
		}
	}
	return result
}

func (m *Manager) Get(id string) (Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return Task{}, false
	}
	return cloneTask(task), true
}

func (m *Manager) GetOrZero(id string) Task {
	task, _ := m.Get(id)
	return task
}

func (m *Manager) Summary() Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := Summary{Total: len(m.tasks)}
	for _, task := range m.tasks {
		switch task.Status {
		case StatusQueued, StatusRunning:
			summary.Active++
		case StatusFailed:
			summary.Failed++
		}
	}
	return summary
}

func (m *Manager) run(id string, run func(context.Context, *Handle) (any, error)) {
	startedAt := time.Now().UTC()
	m.update(id, func(task *Task) {
		task.Status = StatusRunning
		task.StartedAt = &startedAt
	})

	result, err := run(context.Background(), &Handle{manager: m, taskID: id})

	finishedAt := time.Now().UTC()
	m.update(id, func(task *Task) {
		task.FinishedAt = &finishedAt
		task.Result = result
		if err != nil {
			task.Status = StatusFailed
			task.Error = err.Error()
			if task.Message == "" {
				task.Message = "Task failed"
			}
			return
		}
		task.Status = StatusCompleted
		if task.Total > 0 && task.Completed < task.Total {
			task.Completed = task.Total
			task.Percent = 100
		}
		if task.Message == "" {
			task.Message = "Task completed"
		}
	})

	if err != nil {
		m.log.Warn("task: failed", zap.String("id", id), zap.Error(err))
		return
	}
	m.log.Info("task: completed", zap.String("id", id))
}

func (m *Manager) update(id string, apply func(*Task)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return
	}
	apply(task)
}

func (m *Manager) trimLocked() {
	for len(m.order) > m.maxTasks {
		oldestID := m.order[0]
		m.order = m.order[1:]
		delete(m.tasks, oldestID)
	}
}

func (h *Handle) SetMessage(format string, args ...any) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}
	h.manager.update(h.taskID, func(task *Task) {
		task.Message = message
	})
}

func (h *Handle) SetProgress(completed, total int) {
	if completed < 0 {
		completed = 0
	}
	if total < 0 {
		total = 0
	}
	h.manager.update(h.taskID, func(task *Task) {
		task.Completed = completed
		task.Total = total
		task.Percent = percent(completed, total)
	})
}

func (h *Handle) Increment(total int) {
	h.manager.update(h.taskID, func(task *Task) {
		task.Completed++
		if total > 0 {
			task.Total = total
		}
		task.Percent = percent(task.Completed, task.Total)
	})
}

func percent(completed, total int) float64 {
	if total <= 0 {
		return 0
	}
	if completed > total {
		completed = total
	}
	return float64(completed) / float64(total) * 100
}

func cloneTask(task *Task) Task {
	if task == nil {
		return Task{}
	}
	copy := *task
	copy.Metadata = cloneMap(task.Metadata)
	return copy
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	out := make(map[string]any, len(input))
	for _, key := range keys {
		out[key] = input[key]
	}
	return out
}
