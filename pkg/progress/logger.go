package progress

import (
	"fmt"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/task"
	"go.uber.org/zap"
)

type Logger struct {
	taskID  string
	manager *task.Manager
	logger  *zap.SugaredLogger
	id      string
}

func NewLogger(taskID string, tm *task.Manager, logger *zap.SugaredLogger) *Logger {
	return &Logger{
		taskID:  taskID,
		manager: tm,
		logger:  logger,
	}
}

func (p *Logger) SetID(digest string) {
	p.id = digest
}

func (l *Logger) UpdateProgress(action task.ActionType, current, total int64) {
	if total <= 0 || l.id == "" {
		log.Logger.Warnf("Invalid progress data: id=%s, current=%d, total=%d",
			l.id, current, total)
		return
	}

	event := task.Event{
		Type:       task.EventProgress,
		TaskID:     l.taskID,
		Identifier: l.id,
		Progress:   current,
		Total:      total,
		Timestamp:  time.Now(),
		Action:     action,
	}

	l.manager.PublishEvent(l.taskID, event)
}

func (p *Logger) InfolnWithAction(msg any, typ task.EventType) {
	message := fmt.Sprint(msg)
	p.manager.PublishEvent(p.taskID, task.Event{
		Type:    typ,
		Level:   task.LogLevelInfo,
		Message: message,
	})
}

func (p *Logger) Infoln(msg any) {
	println(p.manager, p.taskID, task.LogLevelInfo, msg)
}

func (p *Logger) Infof(format string, args ...any) {
	print(p.manager, p.taskID, task.LogLevelInfo, format, args...)
}

func (p *Logger) Warnln(msg any) {
	println(p.manager, p.taskID, task.LogLevelWarn, msg)
}

func (p *Logger) Warnf(format string, args ...any) {
	print(p.manager, p.taskID, task.LogLevelWarn, format, args...)
}

func (p *Logger) Debugln(msg any) {
	println(p.manager, p.taskID, task.LogLevelDebug, msg)
}

func (p *Logger) Debugf(format string, args ...any) {
	print(p.manager, p.taskID, task.LogLevelDebug, format, args...)
}

func (p *Logger) Errorln(msg any) {
	println(p.manager, p.taskID, task.LogLevelError, msg)
}

func (p *Logger) Errorf(format string, args ...any) {
	print(p.manager, p.taskID, task.LogLevelError, format, args...)
}

func (p *Logger) Wait() {
}

func println(m *task.Manager, id string, level task.LogLevel, arg any) {
	print(m, id, level, "", arg)
}

func print(m *task.Manager, id string, level task.LogLevel, format string, args ...any) {
	message := fmt.Sprint(args[0])
	if format != "" {
		message = fmt.Sprintf(format, args...)
	}
	log.Logger.Debug(message)
	m.PublishEvent(id, task.Event{
		Type:    task.EventLog,
		Level:   level,
		Message: message,
	})
}
