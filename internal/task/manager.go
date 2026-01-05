package task

import (
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	tasks   map[string]*Task
	mu      sync.RWMutex
	cleanup time.Duration
}

func NewManager(cleanupInterval time.Duration) *Manager {
	tm := &Manager{
		tasks:   make(map[string]*Task),
		cleanup: cleanupInterval,
	}
	go tm.cleanupLoop()
	return tm
}

func (tm *Manager) CreateTask(taskType string, metadata map[string]interface{}) *Task {
	task := New(taskType, metadata)
	tm.mu.Lock()
	tm.tasks[task.ID] = task
	tm.mu.Unlock()
	return task
}

func (tm *Manager) GetTask(taskID string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	task, exists := tm.tasks[taskID]
	return task, exists
}

func (tm *Manager) ListTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		taskCopy := &Task{
			ID:        task.ID,
			Type:      task.Type,
			Status:    task.Status,
			StartTime: task.StartTime,
			EndTime:   task.EndTime,
			Progress:  task.Progress,
			Total:     task.Total,
			Error:     task.Error,
			Metadata:  task.Metadata,
		}
		tasks = append(tasks, taskCopy)
	}

	return tasks
}

func (tm *Manager) Subscribe(taskID string, ch chan<- Event) error {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.mu.Lock()
	task.subscribers[ch] = true
	task.mu.Unlock()
	return nil
}

func (tm *Manager) Unsubscribe(taskID string, ch chan<- Event) {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	tm.mu.RUnlock()

	if exists {
		task.mu.Lock()
		delete(task.subscribers, ch)
		task.mu.Unlock()
	}
}

func (tm *Manager) PublishEvent(taskID string, event Event) {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return
	}

	event.TaskID = taskID
	event.Timestamp = time.Now()

	task.mu.RLock()
	for ch := range task.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	task.mu.RUnlock()
}

func (tm *Manager) CompleteTask(taskID string, err error) {
	tm.mu.RLock()
	task, exists := tm.tasks[taskID]
	tm.mu.RUnlock()

	if !exists {
		return
	}

	task.mu.Lock()
	now := time.Now()
	task.EndTime = &now
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else {
		task.Status = TaskStatusCompleted
	}
	task.cancel()
	task.mu.Unlock()

	eventType := EventComplete
	if err != nil {
		eventType = EventError
	}

	tm.PublishEvent(taskID, Event{
		Type:    eventType,
		Message: task.Error,
	})
}

func (tm *Manager) cleanupLoop() {
	ticker := time.NewTicker(tm.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		tm.mu.Lock()
		for id, task := range tm.tasks {
			task.mu.RLock()
			shouldCleanup := task.Status != TaskStatusRunning &&
				task.EndTime != nil &&
				time.Since(*task.EndTime) > tm.cleanup
			task.mu.RUnlock()

			if shouldCleanup {
				delete(tm.tasks, id)
			}
		}
		tm.mu.Unlock()
	}
}
