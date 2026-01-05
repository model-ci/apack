//go:generate easyjson task.go
package task

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

//easyjson:json
type Items struct {
	Tasks []*Task `json:"tasks"`
	Count int     `json:"count"`
}

type TaskStatus string

const (
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCanceled  TaskStatus = "canceled"
)

//easyjson:json
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "push", "pull"
	Status      TaskStatus             `json:"status"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Progress    int64                  `json:"progress"`
	Total       int64                  `json:"total"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	subscribers map[chan<- Event]bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func New(taskType string, metadata map[string]interface{}) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ID:          uuid.New().String(),
		Type:        taskType,
		Status:      TaskStatusRunning,
		StartTime:   time.Now(),
		Metadata:    metadata,
		subscribers: make(map[chan<- Event]bool),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (t *Task) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status == TaskStatusRunning {
		t.Status = TaskStatusCanceled
		if t.cancel != nil {
			t.cancel()
		}
	}
}

func (t *Task) Context() context.Context {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ctx
}

func (t *Task) IsCanceled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status == TaskStatusCanceled
}
