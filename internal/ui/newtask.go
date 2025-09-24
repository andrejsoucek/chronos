package ui

import (
	"fmt"
	"time"

	"github.com/jroimartin/gocui"
)

type taskEditor struct {
	ui *ReportUI
}

func (e *taskEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyEnter:
		return
	case key == gocui.KeyEsc:
		return
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		if len(e.ui.newTaskBuffer) > 0 {
			e.ui.newTaskBuffer = e.ui.newTaskBuffer[:len(e.ui.newTaskBuffer)-1]
		}
	case ch != 0:
		if ch >= 32 && ch <= 126 { // Printable ASCII characters
			e.ui.newTaskBuffer += string(ch)
		}
	}
}

func (ui *ReportUI) confirmNewTask(g *gocui.Gui, v *gocui.View) error {
	if ui.newTaskBuffer == "" {
		ui.logError("Task name cannot be empty")
		return nil
	}

	for _, existingTask := range ui.taskNames {
		if existingTask == ui.newTaskBuffer {
			ui.logError(fmt.Sprintf("Task '%s' already exists", ui.newTaskBuffer))
			ui.isAddingTask = false
			ui.newTaskBuffer = ""
			if _, err := g.SetCurrentView("table"); err != nil {
				return err
			}
			return nil
		}
	}

	ui.taskNames = append(ui.taskNames, ui.newTaskBuffer)

	if ui.taskDayMap[ui.newTaskBuffer] == nil {
		ui.taskDayMap[ui.newTaskBuffer] = make(map[int]time.Duration)
	}
	if ui.taskDayIDMap[ui.newTaskBuffer] == nil {
		ui.taskDayIDMap[ui.newTaskBuffer] = make(map[int]string)
	}

	ui.selectedCell.TaskIndex = len(ui.taskNames) - 1
	ui.selectedCell.DayIndex = 0

	ui.logInfo(fmt.Sprintf("Added new task: '%s'", ui.newTaskBuffer))

	ui.isAddingTask = false
	ui.newTaskBuffer = ""

	if _, err := g.SetCurrentView("table"); err != nil {
		return err
	}

	return nil
}
