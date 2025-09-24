package action

import (
	"time"

	"github.com/andrejsoucek/chronos/pkg/clockify"
)

func LogTime(c *clockify.Clockify, projectId string, duration time.Duration, taskName string) error {
	te := &clockify.TimeEntry{
		Time:        time.Now(),
		Duration:    duration,
		Description: taskName,
		ProjectID:   projectId,
	}
	_, err := c.LogTime(te)
	return err
}
