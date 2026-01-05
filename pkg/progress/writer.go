package progress

import (
	"sync"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/task"
)

type ProgressWriter struct {
	log *Logger

	total     int64
	written   int64
	completed bool
	id        string
	writerID  int32
	action    task.ActionType
	mu        sync.RWMutex
}

func NewProgressWriter(log *Logger, action task.ActionType, id string, total int64) *ProgressWriter {
	return &ProgressWriter{
		log:       log,
		id:        id,
		total:     total,
		written:   0,
		completed: false,
		action:    action,
	}
}

func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	pw.mu.Lock()
	pw.written += int64(n)

	if !pw.completed && pw.written >= pw.total {
		pw.completed = true
		pw.mu.Unlock()

		log.Logger.Infof("Writer #%d (%s): COMPLETED! written=%d, total=%d",
			pw.writerID, pw.id, pw.written, pw.total)

		pw.log.SetID(pw.id)
		pw.log.InfolnWithAction("Layer transfer completed", task.EventProgress)
	} else {
		pw.mu.Unlock()
	}

	pw.log.SetID(pw.id)
	pw.log.UpdateProgress(pw.action, pw.written, pw.total)

	return n, nil
}

func (pw *ProgressWriter) MarkCompleted() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if !pw.completed {
		pw.completed = true

		pw.log.SetID(pw.id)
		pw.log.UpdateProgress(pw.action, pw.written, pw.total)
		log.Logger.Infof("%s Writer #%d (%s): SKIPPED (already exists), written: %d, total: %d", pw.action, pw.writerID, pw.id, pw.written, pw.total)
	}
}

func (pw *ProgressWriter) IsCompleted() bool {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	return pw.completed
}
