package ui

import "github.com/jroimartin/gocui"

func (ui *ReportUI) setKeybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	// Global keybindings for navigation (work regardless of edit mode)
	if err := g.SetKeybinding("table", gocui.KeyArrowUp, gocui.ModNone, ui.moveCursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("table", gocui.KeyArrowDown, gocui.ModNone, ui.moveCursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("table", gocui.KeyArrowLeft, gocui.ModNone, ui.moveCursorLeft); err != nil {
		return err
	}
	if err := g.SetKeybinding("table", gocui.KeyArrowRight, gocui.ModNone, ui.moveCursorRight); err != nil {
		return err
	}
	if err := g.SetKeybinding("table", gocui.KeyEnter, gocui.ModNone, ui.editCell); err != nil {
		return err
	}

	// Try multiple Esc key approaches - these may not work in all terminals
	_ = g.SetKeybinding("table", gocui.KeyEsc, gocui.ModNone, ui.cancelEdit)

	// Add 'q' as an alternative to escape (this we know works)
	if err := g.SetKeybinding("table", 'q', gocui.ModNone, ui.handleQ); err != nil {
		return err
	}

	// Global backspace
	if err := g.SetKeybinding("table", gocui.KeyBackspace, gocui.ModNone, ui.handleBackspace); err != nil {
		return err
	}
	if err := g.SetKeybinding("table", gocui.KeyBackspace2, gocui.ModNone, ui.handleBackspace); err != nil {
		return err
	}

	// Add individual key bindings for common duration input characters
	chars := "0123456789hms:"
	for _, ch := range chars {
		if err := g.SetKeybinding("table", ch, gocui.ModNone, ui.makeCharHandler(ch)); err != nil {
			return err
		}
	}

	// Add scroll keybindings for log area
	if err := g.SetKeybinding("log", gocui.KeyPgup, gocui.ModNone, ui.scrollLogUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("log", gocui.KeyPgdn, gocui.ModNone, ui.scrollLogDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("log", gocui.KeyArrowUp, gocui.ModNone, ui.scrollLogUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("log", gocui.KeyArrowDown, gocui.ModNone, ui.scrollLogDown); err != nil {
		return err
	}

	// Add keybinding to focus log area with 'L' key
	if err := g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, ui.focusLog); err != nil {
		return err
	}
	// Add keybinding to focus table area with 'T' key
	if err := g.SetKeybinding("", gocui.KeyCtrlT, gocui.ModNone, ui.focusTable); err != nil {
		return err
	}

	// Add keybinding to add new task with Ctrl+N
	if err := g.SetKeybinding("", gocui.KeyCtrlN, gocui.ModNone, ui.addNewTask); err != nil {
		return err
	}

	// Add keybinding to delete entry with Ctrl+D
	if err := g.SetKeybinding("table", gocui.KeyCtrlD, gocui.ModNone, ui.deleteEntry); err != nil {
		return err
	}

	// Add keybinding to refresh data with Ctrl+R
	if err := g.SetKeybinding("", gocui.KeyCtrlR, gocui.ModNone, ui.refreshData); err != nil {
		return err
	}

	// Keybindings for task popup
	if err := g.SetKeybinding("taskPopup", gocui.KeyEnter, gocui.ModNone, ui.confirmNewTask); err != nil {
		return err
	}
	if err := g.SetKeybinding("taskPopup", gocui.KeyEsc, gocui.ModNone, ui.cancelAddTask); err != nil {
		return err
	}

	return nil
}

func (ui *ReportUI) moveCursorUp(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil
	}
	if ui.selectedCell.TaskIndex > 0 {
		ui.selectedCell.TaskIndex--
	}
	return nil
}

func (ui *ReportUI) moveCursorDown(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil
	}
	if ui.selectedCell.TaskIndex < len(ui.taskNames)-1 {
		ui.selectedCell.TaskIndex++
	}
	return nil
}

func (ui *ReportUI) moveCursorLeft(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil
	}
	if ui.selectedCell.DayIndex > 0 {
		ui.selectedCell.DayIndex--
	}
	return nil
}

func (ui *ReportUI) moveCursorRight(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil
	}
	if ui.selectedCell.DayIndex < len(ui.days)-1 {
		ui.selectedCell.DayIndex++
	}
	return nil
}

func (ui *ReportUI) cancelEdit(g *gocui.Gui, v *gocui.View) error {
	if ui.isAddingTask {
		ui.isAddingTask = false
		ui.newTaskBuffer = ""
		ui.logInfo("Add new task cancelled")
		return nil
	}
	ui.isEditing = false
	ui.editBuffer = ""
	ui.logInfo("Edit cancelled")
	return nil
}

func (ui *ReportUI) handleQ(g *gocui.Gui, v *gocui.View) error {
	if ui.isAddingTask {
		// If adding task, 'q' cancels the add
		return ui.cancelEdit(g, v)
	}
	if ui.isEditing {
		// If editing, 'q' cancels the edit
		return ui.cancelEdit(g, v)
	}
	// If not editing, 'q' quits the application
	return gocui.ErrQuit
}

func (ui *ReportUI) scrollLogUp(g *gocui.Gui, v *gocui.View) error {
	logView, err := g.View("log")
	if err != nil {
		return err
	}
	ox, oy := logView.Origin()
	if oy > 0 {
		logView.SetOrigin(ox, oy-1)
	}
	return nil
}

func (ui *ReportUI) scrollLogDown(g *gocui.Gui, v *gocui.View) error {
	logView, err := g.View("log")
	if err != nil {
		return err
	}
	ox, oy := logView.Origin()
	lines := len(logView.BufferLines())
	_, viewHeight := logView.Size()

	maxScroll := lines - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	if oy < maxScroll {
		logView.SetOrigin(ox, oy+1)
	}
	return nil
}

func (ui *ReportUI) focusLog(g *gocui.Gui, v *gocui.View) error {
	if !ui.isEditing && !ui.isAddingTask {
		_, err := g.SetCurrentView("log")
		return err
	}
	return nil
}

func (ui *ReportUI) focusTable(g *gocui.Gui, v *gocui.View) error {
	if !ui.isEditing && !ui.isAddingTask {
		_, err := g.SetCurrentView("table")
		return err
	}
	return nil
}

func (ui *ReportUI) handleBackspace(g *gocui.Gui, v *gocui.View) error {
	if ui.isAddingTask {
		if len(ui.newTaskBuffer) > 0 {
			ui.newTaskBuffer = ui.newTaskBuffer[:len(ui.newTaskBuffer)-1]
		}
		return nil
	}
	if !ui.isEditing {
		return nil
	}

	if len(ui.editBuffer) > 0 {
		ui.editBuffer = ui.editBuffer[:len(ui.editBuffer)-1]
	}
	return nil
}

func (ui *ReportUI) makeCharHandler(ch rune) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		if ui.isAddingTask {
			ui.newTaskBuffer += string(ch)
			return nil
		}
		if !ui.isEditing {
			// If not editing and it's 'q', quit
			if ch == 'q' {
				return gocui.ErrQuit
			}
			return nil
		}
		ui.editBuffer += string(ch)
		return nil
	}
}

func (ui *ReportUI) addNewTask(g *gocui.Gui, v *gocui.View) error {
	if ui.isEditing || ui.isAddingTask {
		return nil // Don't allow if already in edit mode
	}

	ui.isAddingTask = true
	ui.newTaskBuffer = ""
	ui.logInfo("Enter new task name (press Enter to confirm, Esc to cancel)")
	return nil
}

func (ui *ReportUI) cancelAddTask(g *gocui.Gui, v *gocui.View) error {
	ui.isAddingTask = false
	ui.newTaskBuffer = ""
	ui.logInfo("Add new task cancelled")

	if _, err := g.SetCurrentView("table"); err != nil {
		return err
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
