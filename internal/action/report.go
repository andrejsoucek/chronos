package action

import (
	"time"

	"github.com/andrejsoucek/chronos/internal/ui"
	"github.com/andrejsoucek/chronos/pkg/clockify"
)

func ShowReport(c *clockify.Clockify, from time.Time, to time.Time) error {
	data, err := c.GetReport(from, to)
	if err != nil {
		return err
	}

	ui.RenderReport(from.Month(), data)
	return nil
}
