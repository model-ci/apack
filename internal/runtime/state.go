//go:generate easyjson -all state.go
package runtime

import (
	"fmt"
	"time"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	OCIAnnotationRuntimeState = "org.opencontainers.runtime.state.v1+json"
)

type States struct {
	Count int
	Items []State
}

type State struct {
	oci.Descriptor

	ID         string
	ModelImage string
	CreateAt   time.Time
	Status     string
	Endpoints  []string
	Names      []string
}

func (s *State) UpdateStatus(st statusType, last time.Time) {
	s.Status = FormatStatus(st, time.Now().Sub(last), "")
}

type statusType string

const (
	StatusUp         statusType = "Up"
	StatusExited     statusType = "Exited"
	StatusCreated    statusType = "Created"
	StatusRestarting statusType = "Restarting"
	StatusDead       statusType = "Dead"
)

func humanizeDuration(duration time.Duration, withAgo bool) string {
	var suffix string
	if withAgo {
		suffix = " ago"
	}
	if duration < time.Minute {
		secs := int(duration.Seconds())
		if secs <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%d seconds%s", secs, suffix)
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%d minutes%s", mins, suffix)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hours%s", hours, suffix)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d days%s", days, suffix)
	}
}

func FormatStatus(status statusType, t time.Duration, extra string) string {
	switch status {
	case StatusUp:
		if extra != "" {
			return fmt.Sprintf("Up %s %s", humanizeDuration(t, false), extra)
		}
		return fmt.Sprintf("Up %s", humanizeDuration(t, false))
	case StatusExited:
		if extra != "" {
			return fmt.Sprintf("Exited %s %s", extra, humanizeDuration(t, true))
		}
		return fmt.Sprintf("Exited %s", humanizeDuration(t, true))
	case StatusRestarting:
		if extra != "" {
			return fmt.Sprintf("Restarting %s %s", extra, humanizeDuration(t, true))
		}
		return fmt.Sprintf("Restarting %s", humanizeDuration(t, true))
	case StatusCreated:
		return "Created"
	case StatusDead:
		return "Dead"
	default:
		return "Unknown"
	}
}
