package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/andrejsoucek/chronos/pkg/clockify"
	"github.com/andrejsoucek/chronos/pkg/datetimeutils"
	"github.com/jroimartin/gocui"
)

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
	sort.Strings(ui.taskNames)

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
