package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/andrejsoucek/chronos/pkg/clockify"
	"github.com/andrejsoucek/chronos/pkg/datetimeutils"
	"github.com/jroimartin/gocui"
)

const (
	taskColumnWidth = 50
	dayColumnWidth  = 8
	ellipsisLength  = 3
)

func RenderReport(month time.Month, data []clockify.ReportTimeEntry) {
	reportMonth := time.Date(time.Now().Year(), month, 1, 0, 0, 0, 0, time.UTC)
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		slog.Error("Failed to create GUI", "error", err)
		return
	}
	defer g.Close()

	g.SetManagerFunc(func(g *gocui.Gui) error {
		return layout(g, data, reportMonth)
	})

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		slog.Error("Failed to set keybinding", "error", err)
		return
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		slog.Error("GUI main loop failed", "error", err)
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func layout(g *gocui.Gui, data []clockify.ReportTimeEntry, reportMonth time.Time) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("table", 0, 0, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		// Create the table content
		tableContent := buildTable(data, reportMonth)

		fmt.Fprint(v, tableContent)
		v.Title = fmt.Sprintf(" Time Report - %s ", time.Now().Format("January 2006"))
		v.Wrap = false
	}

	return nil
}

func buildTable(data []clockify.ReportTimeEntry, reportMonth time.Time) string {
	var sb strings.Builder

	// Group data by task and create task list
	taskDayMap, taskNames := groupDataByTaskAndDay(data)
	days := datetimeutils.DaysInMonth(reportMonth)

	buildHeaders(&sb, reportMonth, days)
	addSeparatorLine(&sb, days)
	buildTaskRows(&sb, taskNames, taskDayMap, days)
	addTotalsRow(&sb, taskNames, taskDayMap, days)

	sb.WriteString("\n\nPress Ctrl+C to exit")

	return sb.String()
}

func groupDataByTaskAndDay(data []clockify.ReportTimeEntry) (map[string]map[int]time.Duration, map[string]bool) {
	taskDayMap := make(map[string]map[int]time.Duration)
	taskNames := make(map[string]bool)

	for _, entry := range data {
		task := entry.Description
		if task == "" {
			task = "Unnamed Task"
		}
		taskNames[task] = true

		if taskDayMap[task] == nil {
			taskDayMap[task] = make(map[int]time.Duration)
		}

		day := entry.TimeInterval.Start.Day()
		duration := entry.TimeInterval.End.Sub(entry.TimeInterval.Start)
		taskDayMap[task][day] += duration
	}

	return taskDayMap, taskNames
}

func buildHeaders(sb *strings.Builder, reportMonth time.Time, days []int) {
	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, ""))
	sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, ""))
	sb.WriteString(" | ")
	for _, day := range days {
		sb.WriteString(fmt.Sprintf("%*d", dayColumnWidth, day))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, "Task"))
	sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "Total"))
	sb.WriteString(" | ")
	for _, day := range days {
		date := time.Date(reportMonth.Year(), reportMonth.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := date.Format("Mon")
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, dayName[:ellipsisLength]))
	}
	sb.WriteString("\n")
}

func addSeparatorLine(sb *strings.Builder, days []int) {
	sb.WriteString(strings.Repeat("-", taskColumnWidth))
	sb.WriteString(strings.Repeat("-", dayColumnWidth))
	sb.WriteString("-+-")
	for range days {
		sb.WriteString(strings.Repeat("-", dayColumnWidth))
	}
	sb.WriteString("\n")
}

func buildTaskRows(sb *strings.Builder, taskNames map[string]bool, taskDayMap map[string]map[int]time.Duration, days []int) {
	for task := range taskNames {
		sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, truncateString(task, taskColumnWidth)))

		taskTotal := time.Duration(0)
		for _, day := range days {
			if duration, exists := taskDayMap[task][day]; exists {
				taskTotal += duration
			}
		}

		if taskTotal > 0 {
			sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, datetimeutils.ShortDur(taskTotal)))
		} else {
			sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
		}
		sb.WriteString(" | ")

		for _, day := range days {
			duration := taskDayMap[task][day]
			if duration > 0 {
				sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, datetimeutils.ShortDur(duration)))
			} else {
				sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
			}
		}
		sb.WriteString("\n")
	}
}

func addTotalsRow(sb *strings.Builder, taskNames map[string]bool, taskDayMap map[string]map[int]time.Duration, days []int) {
	addSeparatorLine(sb, days)
	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, "TOTAL"))

	grandTotal := time.Duration(0)
	for task := range taskNames {
		for _, day := range days {
			if duration, exists := taskDayMap[task][day]; exists {
				grandTotal += duration
			}
		}
	}

	if grandTotal > 0 {
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, datetimeutils.ShortDur(grandTotal)))
	} else {
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
	}
	sb.WriteString(" | ")

	for _, day := range days {
		totalDuration := time.Duration(0)
		for task := range taskNames {
			if duration, exists := taskDayMap[task][day]; exists {
				totalDuration += duration
			}
		}

		if totalDuration > 0 {
			sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, datetimeutils.ShortDur(totalDuration)))
		} else {
			sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
		}
	}
	sb.WriteString("\n")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= ellipsisLength {
		return s[:maxLen]
	}
	return s[:maxLen-ellipsisLength] + "..."
}
