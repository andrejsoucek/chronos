package action

import (
	"time"

	"github.com/andrejsoucek/chronos/pkg/clockify"
)

func LogTime(c *clockify.Clockify, projectId string, duration time.Duration, taskName string) error {
	te := &clockify.TimeEntry{
		Duration:    duration,
		Description: taskName,
		ProjectID:   projectId,
	}
	return c.LogTime(te)
}
