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
	taskColumnWidth = 35
	dayColumnWidth  = 8
	ellipsisLength  = 3
	asciiEsc        = 27
)

type CellPosition struct {
	TaskIndex int
	DayIndex  int
	IsTotal   bool
}

type ReportUI struct {
	clockifyClient   *clockify.Clockify
	data             []clockify.ReportTimeEntry
	reportMonth      time.Time
	taskDayMap       map[string]map[int]time.Duration
	taskDayIDMap     map[string]map[int]string // Maps task+day to time entry ID for existing entries
	taskNames        []string
	days             []int
	selectedCell     CellPosition
	isEditing        bool
	editBuffer       string
	logMessages      []string // Changed from single string to slice
	projectId        string
	isAddingTask     bool   // Track if we're in "add new task" mode
	newTaskBuffer    string // Buffer for new task name input
	shouldAutoScroll bool   // Flag to control when to auto-scroll log
}

func RenderReport(c *clockify.Clockify, projectId string, month time.Month, data []clockify.ReportTimeEntry) {
	reportMonth := time.Date(time.Now().Year(), month, 1, 0, 0, 0, 0, time.UTC)
	g, err := gocui.NewGui(gocui.OutputNormal)
	g.InputEsc = true
	g.Highlight = true
	g.SelFgColor = gocui.ColorGreen

	if err != nil {
		slog.Error("Failed to create GUI", "error", err)
		return
	}
	defer g.Close()

	ui := &ReportUI{
		clockifyClient: c,
		data:           data,
		reportMonth:    reportMonth,
		days:           datetimeutils.DaysInMonth(reportMonth),
		projectId:      projectId,
	}

	taskDayMap, taskNamesMap, taskDayIDMap := groupDataByTaskAndDay(data)
	ui.taskDayMap = taskDayMap
	ui.taskDayIDMap = taskDayIDMap

	ui.taskNames = make([]string, 0, len(taskNamesMap))
	for task := range taskNamesMap {
		ui.taskNames = append(ui.taskNames, task)
	}

	g.SetManagerFunc(func(g *gocui.Gui) error {
		return ui.layout(g)
	})

	if err := ui.setKeybindings(g); err != nil {
		slog.Error("Failed to set keybindings", "error", err)
		return
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		slog.Error("GUI main loop failed", "error", err)
	}
}

func (ui *ReportUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	logHeight := 5
	tableHeight := maxY - logHeight - 1

	if v, err := g.SetView("table", 0, 0, maxX-1, tableHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Time Report - %s ", ui.reportMonth.Format("January 2006"))
		v.Wrap = false

		if _, err := g.SetCurrentView("table"); err != nil {
			return err
		}
	}

	if v, err := g.SetView("log", 0, tableHeight+1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = true
		v.Autoscroll = false
	}

	if ui.isAddingTask {
		popupWidth := 50
		popupHeight := 5
		x0 := (maxX - popupWidth) / 2
		y0 := (maxY - popupHeight) / 2
		x1 := x0 + popupWidth
		y1 := y0 + popupHeight

		if v, err := g.SetView("taskPopup", x0, y0, x1, y1); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			v.Title = " Add New Task "
			v.Wrap = true
			v.Editable = true
			v.Editor = &taskEditor{ui: ui}

			if _, err := g.SetCurrentView("taskPopup"); err != nil {
				return err
			}
		}

		if v, err := g.View("taskPopup"); err == nil {
			v.Clear()
			fmt.Fprintf(v, "Task name: %s\n\nPress Enter to confirm, Esc to cancel", ui.newTaskBuffer)
			v.SetCursor(len(ui.newTaskBuffer), 0)
		}
	} else {
		if err := g.DeleteView("taskPopup"); err != nil && err != gocui.ErrUnknownView {
			return err
		}
	}

	if v, err := g.View("table"); err == nil {
		v.Clear()
		tableContent := ui.buildTable()
		fmt.Fprint(v, tableContent)
	}

	if v, err := g.View("log"); err == nil {
		v.Clear()
		v.Title = " Log "
		if len(ui.logMessages) > 0 {
			for _, msg := range ui.logMessages {
				fmt.Fprintln(v, msg)
			}
			// Only auto-scroll to bottom when new messages are added
			if ui.shouldAutoScroll {
				ui.scrollToBottomOfLog(g)
				ui.shouldAutoScroll = false
			}
		}
	}

	return nil
}

func (ui *ReportUI) editCell(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing {
		return ui.saveEdit(g, v)
	}

	ui.isEditing = true
	task := ui.taskNames[ui.selectedCell.TaskIndex]
	day := ui.days[ui.selectedCell.DayIndex]

	if duration, exists := ui.taskDayMap[task][day]; exists && duration > 0 {
		ui.editBuffer = datetimeutils.ShortDur(duration)
	} else {
		ui.editBuffer = ""
	}

	return nil
}

func (ui *ReportUI) saveEdit(g *gocui.Gui, v *gocui.View) error {
	if ui.editBuffer == "" {
		ui.isEditing = false
		ui.clearLog()
		return nil
	}

	duration, err := time.ParseDuration(ui.editBuffer)
	if err != nil {
		// Invalid duration format - show error and stay in exit edit mode
		ui.logError(fmt.Sprintf("Invalid duration format: %s", ui.editBuffer))
		ui.isEditing = false
		ui.editBuffer = ""
		return nil
	}

	task := ui.taskNames[ui.selectedCell.TaskIndex]
	day := ui.days[ui.selectedCell.DayIndex]

	currentTime := time.Now()
	targetDate := time.Date(
		ui.reportMonth.Year(),
		ui.reportMonth.Month(),
		day,
		currentTime.Hour(),
		currentTime.Minute(),
		currentTime.Second(),
		0,
		time.UTC,
	)

	var existingID string
	if ui.taskDayIDMap[task] != nil {
		existingID = ui.taskDayIDMap[task][day]
	}

	if ui.taskDayMap[task] == nil {
		ui.taskDayMap[task] = make(map[int]time.Duration)
	}
	ui.taskDayMap[task][day] = duration

	// Edit existing time entry
	if existingID != "" {
		timeEntry := &clockify.TimeEntry{
			Time:        targetDate,
			Duration:    duration,
			Description: task,
			ProjectID:   ui.projectId,
		}

		ui.logInfo(
			fmt.Sprintf(
				"Attempting to update existing entry (ID %s): %s for '%s' on %s",
				existingID,
				duration,
				task,
				targetDate.Format("2006-01-02"),
			),
		)

		if err := ui.clockifyClient.EditLog(existingID, timeEntry); err != nil {
			ui.logError(fmt.Sprintf("Failed to update time entry: %v", err))
			ui.editBuffer = "Update failed"
			return nil
		}

		ui.logInfo(fmt.Sprintf("Successfully updated %s for %s on day %d (ID: %s)", duration, task, day, existingID))
	} else {
		timeEntry := &clockify.TimeEntry{
			Time:        targetDate,
			Duration:    duration,
			Description: task,
			ProjectID:   ui.projectId,
		}

		ui.logInfo(fmt.Sprintf("Attempting to log new entry: %s for '%s' on %s", duration, task, targetDate.Format("2006-01-02")))

		newEntryID, err := ui.clockifyClient.LogTime(timeEntry)
		if err != nil {
			ui.logError(fmt.Sprintf("Failed to save time entry: %v", err))
			ui.editBuffer = "Save failed"
			return nil
		}

		if ui.taskDayIDMap[task] == nil {
			ui.taskDayIDMap[task] = make(map[int]string)
		}
		ui.taskDayIDMap[task][day] = newEntryID

		ui.logInfo(fmt.Sprintf("Successfully logged %s for %s on day %d (ID: %s)", duration, task, day, newEntryID))
	}

	ui.isEditing = false
	ui.editBuffer = ""
	return nil
}

func (ui *ReportUI) buildTable() string {
	var sb strings.Builder

	ui.buildHeaders(&sb)
	ui.addSeparatorLine(&sb)
	ui.buildTaskRows(&sb)
	ui.addTotalsRow(&sb)

	sb.WriteString(
		"\n\n\033[1mArrow keys\033[0m: Navigate | \033[1mEnter\033[0m: Edit/Save | \033[1mQ/Esc\033[0m: Cancel | \033[1mCtrl+N\033[0m: Add new task | \033[1mCtrl+D\033[0m: Delete entry | \033[1mCtrl+R\033[0m: Refresh the table | \033[1mCtrl+L\033[0m: Focus log | \033[1mCtrl+T\033[0m: Focus table | \033[1mCtrl+C\033[0m: Exit",
	)
	sb.WriteString("\nDuration format: 1h30m, 2h, 45m, etc.")

	return sb.String()
}

func (ui *ReportUI) buildHeaders(sb *strings.Builder) {
	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, ""))
	sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, ""))
	sb.WriteString(" | ")
	for _, day := range ui.days {
		sb.WriteString(fmt.Sprintf("%*d", dayColumnWidth, day))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, "Task"))
	sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "Total"))
	sb.WriteString(" | ")
	for _, day := range ui.days {
		date := time.Date(ui.reportMonth.Year(), ui.reportMonth.Month(), day, 0, 0, 0, 0, time.UTC)
		dayName := date.Format("Mon")
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, dayName[:ellipsisLength]))
	}
	sb.WriteString("\n")
}

func (ui *ReportUI) addSeparatorLine(sb *strings.Builder) {
	// Use box-drawing characters for a fully connected line
	sb.WriteString(strings.Repeat("─", taskColumnWidth))
	sb.WriteString(strings.Repeat("─", dayColumnWidth))
	sb.WriteString("─┼─") // horizontal with cross junction
	for range ui.days {
		sb.WriteString(strings.Repeat("─", dayColumnWidth))
	}
	sb.WriteString("\n")
}

func (ui *ReportUI) buildTaskRows(sb *strings.Builder) {
	for taskIndex, task := range ui.taskNames {
		sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, truncateString(task, taskColumnWidth)))

		// Add task total column
		taskTotal := ui.calculateTaskTotal(task)
		ui.appendDurationCell(sb, taskTotal)
		sb.WriteString(" | ")

		// Add daily columns
		for dayIndex, day := range ui.days {
			duration := ui.taskDayMap[task][day]
			isSelected := ui.selectedCell.TaskIndex == taskIndex && ui.selectedCell.DayIndex == dayIndex
			ui.appendEditableCell(sb, duration, isSelected)
		}
		sb.WriteString("\n")
	}
}

func (ui *ReportUI) calculateTaskTotal(task string) time.Duration {
	total := time.Duration(0)
	for _, day := range ui.days {
		if duration, exists := ui.taskDayMap[task][day]; exists {
			total += duration
		}
	}
	return total
}

func (ui *ReportUI) appendDurationCell(sb *strings.Builder, duration time.Duration) {
	if duration > 0 {
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, datetimeutils.ShortDur(duration)))
	} else {
		sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
	}
}

func (ui *ReportUI) appendEditableCell(sb *strings.Builder, duration time.Duration, isSelected bool) {
	var cellContent string
	if ui.isEditing && isSelected {
		cellContent = "[" + ui.editBuffer + "]"
	} else if duration > 0 {
		cellContent = datetimeutils.ShortDur(duration)
	} else {
		cellContent = "-"
	}

	if isSelected && !ui.isEditing {
		cellContent = ">" + cellContent + "<"
	}

	sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, cellContent))
}

func (ui *ReportUI) addTotalsRow(sb *strings.Builder) {
	ui.addSeparatorLine(sb)
	sb.WriteString(fmt.Sprintf("%-*s", taskColumnWidth, "TOTAL"))

	grandTotal := time.Duration(0)
	for _, task := range ui.taskNames {
		for _, day := range ui.days {
			if duration, exists := ui.taskDayMap[task][day]; exists {
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

	for _, day := range ui.days {
		totalDuration := time.Duration(0)
		for _, task := range ui.taskNames {
			if duration, exists := ui.taskDayMap[task][day]; exists {
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

func groupDataByTaskAndDay(data []clockify.ReportTimeEntry) (map[string]map[int]time.Duration, map[string]bool, map[string]map[int]string) {
	taskDayMap := make(map[string]map[int]time.Duration)
	taskNames := make(map[string]bool)
	taskDayIDMap := make(map[string]map[int]string)

	for _, entry := range data {
		task := entry.Description
		if task == "" {
			task = "Unnamed Task"
		}
		taskNames[task] = true

		if taskDayMap[task] == nil {
			taskDayMap[task] = make(map[int]time.Duration)
		}
		if taskDayIDMap[task] == nil {
			taskDayIDMap[task] = make(map[int]string)
		}

		day := entry.TimeInterval.Start.Day()
		duration := entry.TimeInterval.End.Sub(entry.TimeInterval.Start)

		// Since each day should only have one task entry, we sum up durations
		// and keep the last entry's ID (this represents all entries for this task+day)
		taskDayMap[task][day] += duration
		taskDayIDMap[task][day] = entry.ID
	}

	return taskDayMap, taskNames, taskDayIDMap
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

func (ui *ReportUI) deleteEntry(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil
	}

	task := ui.taskNames[ui.selectedCell.TaskIndex]
	day := ui.days[ui.selectedCell.DayIndex]

	var existingID string
	if ui.taskDayIDMap[task] != nil {
		existingID = ui.taskDayIDMap[task][day]
	}

	if existingID == "" {
		ui.logError("No time entry to delete at this position")
		return nil
	}

	currentDuration := ui.taskDayMap[task][day]

	ui.logInfo(fmt.Sprintf("Attempting to delete entry (ID %s): %s for '%s' on day %d",
		existingID, datetimeutils.ShortDur(currentDuration), task, day))

	if err := ui.clockifyClient.DeleteLog(existingID); err != nil {
		ui.logError(fmt.Sprintf("Failed to delete time entry: %v", err))
		return nil
	}

	delete(ui.taskDayMap[task], day)
	delete(ui.taskDayIDMap[task], day)

	ui.logInfo(fmt.Sprintf("Successfully deleted %s for %s on day %d",
		datetimeutils.ShortDur(currentDuration), task, day))

	return nil
}

func (ui *ReportUI) refreshData(g *gocui.Gui, v *gocui.View) error {
	// Don't refresh if we're in edit mode or adding a task
	if ui.isEditing || ui.isAddingTask {
		return nil
	}

	ui.logInfo("Refreshing data...")

	// Calculate the time range for the current report month
	from := time.Date(ui.reportMonth.Year(), ui.reportMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0).Add(-time.Second) // Last second of the month

	// Fetch fresh data from Clockify
	data, err := ui.clockifyClient.GetReport(from, to)
	if err != nil {
		ui.logError(fmt.Sprintf("Failed to refresh data: %v", err))
		return nil
	}

	// Update the UI data
	ui.data = data
	taskDayMap, taskNamesMap, taskDayIDMap := groupDataByTaskAndDay(data)
	ui.taskDayMap = taskDayMap
	ui.taskDayIDMap = taskDayIDMap

	// Update task names
	ui.taskNames = make([]string, 0, len(taskNamesMap))
	for task := range taskNamesMap {
		ui.taskNames = append(ui.taskNames, task)
	}

	// Reset selected cell if it's out of bounds
	if ui.selectedCell.TaskIndex >= len(ui.taskNames) {
		ui.selectedCell.TaskIndex = 0
	}
	if len(ui.days) > 0 && ui.selectedCell.DayIndex >= len(ui.days) {
		ui.selectedCell.DayIndex = 0
	}

	ui.logInfo(fmt.Sprintf("Data refreshed successfully - found %d time entries", len(data)))

	return nil
}
