package action

import (
	"time"

	"github.com/andrejsoucek/chronos/pkg"
)

func LogTime(clockify *pkg.Clockify, projectId string, duration time.Duration, taskName string) error {
	te := &pkg.TimeEntry{
		Duration:    duration,
		Description: taskName,
		ProjectID:   projectId,
	}
	return clockify.LogTime(te)
}
