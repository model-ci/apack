//go:generate easyjson event.go
package task

import (
	"time"
)

type EventType string

const (
	EventProgress EventType = "progress"
	EventLog      EventType = "log"
	EventError    EventType = "error"
	EventComplete EventType = "complete"
	EventCancel   EventType = "cancel"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type ActionType string

const (
	ActionPull    ActionType = "Downloading"
	ActionPush    ActionType = "Uploading"
	ActionBundle  ActionType = "Copying"
	ActionExtract ActionType = "Saving"
	ActionSink    ActionType = "Outputting"
)

//easyjson:json
type Event struct {
	TaskID     string            `json:"task_id"`
	Type       EventType         `json:"type"`
	Action     ActionType        `json:"action,omitempty"`
	Progress   int64             `json:"progress"`
	Total      int64             `json:"total"`
	Message    string            `json:"message"`
	Level      LogLevel          `json:"level"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Identifier string            `json:"identifier,omitempty"`
}
