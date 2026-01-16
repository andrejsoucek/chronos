package ui

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/andrejsoucek/chronos/pkg/clockify"
	"github.com/andrejsoucek/chronos/pkg/datetimeutils"
	"github.com/andrejsoucek/chronos/pkg/gitlab"
	"github.com/andrejsoucek/chronos/pkg/linear"
	"github.com/jroimartin/gocui"
)

const (
	taskColumnWidth    = 35
	dayColumnWidth     = 8
	ellipsisLength     = 3
	asciiEsc           = 27
	weekendPlaceholder = "x" // Used to visually differentiate weekends when empty
)

type CellPosition struct {
	TaskIndex int
	DayIndex  int
	IsTotal   bool
}

type ReportUI struct {
	clockifyClient     *clockify.Clockify
	linearLastActivity []linear.LastActivityItem
	gitlabLastActivity []gitlab.LastActivityItem
	data               []clockify.ReportTimeEntry
	reportMonth        time.Time
	taskDayMap         map[string]map[int]time.Duration
	taskDayIDMap       map[string]map[int]string // Maps task+day to time entry ID for existing entries
	taskNames          []string
	days               []int
	selectedCell       CellPosition
	isEditing          bool
	editBuffer         string
	logMessages        []string // Changed from single string to slice
	projectId          string
	isAddingTask       bool   // Track if we're in "add new task" mode
	newTaskBuffer      string // Buffer for new task name input
	shouldAutoScroll   bool   // Flag to control when to auto-scroll log
}

func RenderReport(
	c *clockify.Clockify,
	projectId string,
	month time.Month,
	data []clockify.ReportTimeEntry,
	linearLastActivity []linear.LastActivityItem,
	gitlabLastActivity []gitlab.LastActivityItem,
) {
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
		clockifyClient:     c,
		linearLastActivity: linearLastActivity,
		gitlabLastActivity: gitlabLastActivity,
		data:               data,
		reportMonth:        reportMonth,
		days:               datetimeutils.DaysInMonth(reportMonth),
		projectId:          projectId,
	}

	taskDayMap, taskNamesMap, taskDayIDMap := groupDataByTaskAndDay(data)
	ui.taskDayMap = taskDayMap
	ui.taskDayIDMap = taskDayIDMap

	ui.taskNames = make([]string, 0, len(taskNamesMap))
	for task := range taskNamesMap {
		ui.taskNames = append(ui.taskNames, task)
	}
	sort.Strings(ui.taskNames)

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

	// Ensure minimum terminal size to avoid layout issues
	if maxY < 60 {
		// Fall back to simple layout for small terminals
		return ui.simpleLayout(g)
	}

	// 4-panel layout
	// Top section: 1/3 of screen height
	topHeight := maxY / 3
	if topHeight < 3 {
		topHeight = 3
	}

	// Split top section in half horizontally
	topMidX := maxX / 2

	// Bottom section calculations
	remainingHeight := maxY - topHeight - 1 // -1 for separator
	logHeight := 6
	helpHeight := 2 // Reserve space for help text
	if logHeight > (remainingHeight-helpHeight)/2 {
		logHeight = (remainingHeight - helpHeight) / 2
	}
	tableHeight := remainingHeight - logHeight - helpHeight - 1 // -1 for separator

	// Top left: Recent Linear Activity
	if v, err := g.SetView("linearActivity", 0, 0, topMidX-1, topHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Recent Linear Activity "
		v.Wrap = true
		v.Autoscroll = false
	}

	// Top right: Recent Git Activity
	if v, err := g.SetView("gitActivity", topMidX, 0, maxX-1, topHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Recent Git Activity "
		v.Wrap = true
		v.Autoscroll = false
	}

	// Time Report Table
	tableStartY := topHeight + 1
	tableEndY := tableStartY + tableHeight - 1
	if v, err := g.SetView("table", 0, tableStartY, maxX-1, tableEndY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = fmt.Sprintf(" Time Report - %s ", ui.reportMonth.Format("January 2006"))
		v.Wrap = false
		v.Autoscroll = true

		if _, err := g.SetCurrentView("table"); err != nil {
			return err
		}
	}

	// Log view
	logStartY := tableEndY + 1
	logEndY := logStartY + logHeight - 1
	if v, err := g.SetView("log", 0, logStartY, maxX-1, logEndY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Log "
		v.Wrap = true
		v.Autoscroll = false
	}

	// Help view at the very bottom
	helpStartY := logEndY + 1
	if v, err := g.SetView("help", 0, helpStartY, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false // No border for help text
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

	// Update content for activity views (only in 4-panel layout)
	if v, err := g.View("linearActivity"); err == nil {
		v.Clear()
		if len(ui.linearLastActivity) > 0 {
			for _, item := range ui.linearLastActivity {
				// Parse the updated_at time and format it
				updatedTime, err := time.Parse(time.RFC3339, item.UpdatedAt)
				var timeStr string
				if err != nil {
					timeStr = item.UpdatedAt // fallback to raw string if parsing fails
				} else {
					timeStr = updatedTime.Format("Jan 2 15:04")
				}

				fmt.Fprintf(v, "%s | %-8s | %s\n", timeStr, item.Identifier, item.Title)
			}
		} else {
			fmt.Fprintln(v, "No recent Linear activity found")
		}
	}

	if v, err := g.View("gitActivity"); err == nil {
		v.Clear()
		if len(ui.gitlabLastActivity) > 0 {
			for _, item := range ui.gitlabLastActivity {
				var name string
				if item.Title != nil {
					name = *item.Title
				} else if item.PushData != nil {
					name = item.PushData.Ref
				} else {
					name = "N/A"
				}

				fmt.Fprintf(v, "%-12s | %s\n", item.Action, name)
			}
		} else {
			fmt.Fprintln(v, "No recent Git activity found")
		}
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

	// Update help view with keyboard shortcuts
	if v, err := g.View("help"); err == nil {
		v.Clear()
		helpText := "\033[1mArrow keys\033[0m: Navigate | \033[1mEnter\033[0m: Edit/Save | " +
			"\033[1mQ/Esc\033[0m: Cancel | \033[1mCtrl+N\033[0m: Add new task | " +
			"\033[1mCtrl+D\033[0m: Delete entry | \033[1mCtrl+R\033[0m: Refresh | " +
			"\033[1mCtrl+T\033[0m: Focus table | \033[1mCtrl+L\033[0m: Focus Linear | " +
			"\033[1mCtrl+G\033[0m: Focus Git | \033[1mCtrl+C\033[0m: Exit"
		fmt.Fprint(v, helpText)
		fmt.Fprint(v, "\nDuration format: 1h30m, 2h, 45m, etc.")
	}

	return nil
}

// simpleLayout provides a fallback 2-panel layout for small terminals
func (ui *ReportUI) simpleLayout(g *gocui.Gui) error {
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
		v.Title = " Log "
		v.Wrap = true
		v.Autoscroll = false
	}

	// Handle popup for adding tasks (same as main layout)
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

	// Update table content
	if v, err := g.View("table"); err == nil {
		v.Clear()
		tableContent := ui.buildTable()
		fmt.Fprint(v, tableContent)
	}

	// Update log content
	if v, err := g.View("log"); err == nil {
		v.Clear()
		v.Title = " Log "
		if len(ui.logMessages) > 0 {
			for _, msg := range ui.logMessages {
				fmt.Fprintln(v, msg)
			}
			if ui.shouldAutoScroll {
				ui.scrollToBottomOfLog(g)
				ui.shouldAutoScroll = false
			}
		}
	}

	return nil
}

func (ui *ReportUI) buildTable() string {
	var sb strings.Builder

	ui.buildHeaders(&sb)
	ui.addSeparatorLine(&sb)
	ui.buildTaskRows(&sb)
	ui.addTotalsRow(&sb)

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
			ui.appendEditableCell(sb, duration, isSelected, ui.isWeekend(day))
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

func (ui *ReportUI) appendEditableCell(sb *strings.Builder, duration time.Duration, isSelected bool, isWeekend bool) {
	var cellContent string
	if ui.isEditing && isSelected {
		cellContent = "[" + ui.editBuffer + "]"
	} else if duration > 0 {
		cellContent = datetimeutils.ShortDur(duration)
	} else {
		if isWeekend {
			cellContent = weekendPlaceholder
		} else {
			cellContent = "-"
		}
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
			if ui.isWeekend(day) {
				sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, weekendPlaceholder))
			} else {
				sb.WriteString(fmt.Sprintf("%*s", dayColumnWidth, "-"))
			}
		}
	}
	sb.WriteString("\n")
}

// isWeekend reports whether the given day of the currently selected month is a weekend (Saturday or Sunday)
func (ui *ReportUI) isWeekend(day int) bool {
	date := time.Date(ui.reportMonth.Year(), ui.reportMonth.Month(), day, 0, 0, 0, 0, time.UTC)
	wd := date.Weekday()
	return wd == time.Saturday || wd == time.Sunday
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
