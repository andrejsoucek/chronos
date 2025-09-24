package action

import (
	"time"

	"github.com/andrejsoucek/chronos/internal/ui"
	"github.com/andrejsoucek/chronos/pkg/clockify"
	"github.com/andrejsoucek/chronos/pkg/gitlab"
	"github.com/andrejsoucek/chronos/pkg/linear"
)

func ShowReport(c *clockify.Clockify, l *linear.Linear, g *gitlab.Gitlab, projectId string, from time.Time, to time.Time) error {
	data, err := c.GetReport(from, to)
	if err != nil {
		return err
	}

	linearLastActivity, err := l.GetLastActivity(from, to)
	if err != nil {
		return err
	}

	gitlabLastActivity, err := g.GetLastActivity(from, to)
	if err != nil {
		return err
	}

	ui.RenderReport(c, projectId, from.Month(), data, linearLastActivity, gitlabLastActivity)
	return nil
}
