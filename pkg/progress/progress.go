package progress

import (
	"sync"
	"sync/atomic"

	"github.com/model-ci/apack/internal/log"
)

type Progress struct {
	writerCount    int32
	completedCount int32
	mu             sync.Mutex
	writers        map[string]*ProgressWriter
}

func NewProgress() *Progress {
	return &Progress{
		writers: make(map[string]*ProgressWriter),
	}
}

func (p *Progress) Add(pw *ProgressWriter) {
	p.mu.Lock()
	defer p.mu.Unlock()

	writerID := atomic.AddInt32(&p.writerCount, 1)
	pw.writerID = writerID

	p.writers[pw.id] = pw

	log.Logger.Infof("Added writer #%d for layer %s", writerID, pw.id)
}

func (p *Progress) CheckAllCompleted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.writers) == 0 {
		return false
	}

	completedCount := 0
	for digest, writer := range p.writers {
		if writer.completed {
			completedCount++
		} else {
			log.Logger.Infof("Writer for layer %s is not completed yet (written: %d/%d)",
				digest, writer.written, writer.total)
		}
	}

	totalWriters := len(p.writers)
	allCompleted := completedCount == totalWriters

	if allCompleted {
		log.Logger.Infof("All %d writers completed successfully", totalWriters)
	} else {
		log.Logger.Infof("Progress: %d/%d writers completed", completedCount, totalWriters)
	}

	return allCompleted
}
