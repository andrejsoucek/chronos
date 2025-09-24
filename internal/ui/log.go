package ui

import (
	"fmt"
	"time"

	"github.com/jroimartin/gocui"
)

func (ui *ReportUI) logError(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] ERROR: %s", timestamp, message)
	ui.logMessages = append(ui.logMessages, logEntry)
	ui.shouldAutoScroll = true
}

func (ui *ReportUI) logInfo(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] INFO: %s", timestamp, message)
	ui.logMessages = append(ui.logMessages, logEntry)
	ui.shouldAutoScroll = true
}

func (ui *ReportUI) clearLog() {
	ui.logMessages = nil
	ui.shouldAutoScroll = true
}

// scrollToBottomOfLog moves the log view to show the latest messages.
func (ui *ReportUI) scrollToBottomOfLog(g *gocui.Gui) {
	if v, err := g.View("log"); err == nil && len(ui.logMessages) > 0 {
		_, maxY := v.Size()
		if len(ui.logMessages) > maxY {
			v.SetOrigin(0, len(ui.logMessages)-maxY)
		}
	}
}
