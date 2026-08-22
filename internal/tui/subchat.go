package tui

import (
	"fmt"
	"strings"
	"time"

	"rune/internal/sessions"
)

// subchatState manages the drill-in view for a specialist's child session.
// When active, the transcript body swaps to show the child session's events
// instead of the parent's. ArrowUp/Esc pops back to the parent view.
type subchatState struct {
	// active is true when the transcript is showing a child session.
	active bool
	// childSessionID is the session being viewed.
	childSessionID string
	// childSessionTitle is the display title for the subchat nav bar.
	childSessionTitle string
	// parentScrollOffset preserves the chat scroll position so popping back
	// returns to the same view.
	parentScrollOffset int
	// childRows are the rehydrated transcript rows from the child session.
	childRows []transcriptRow
}

// enter loads the child session's events and activates the subchat view.
// It returns an error message string if the session could not be loaded.
func (s *subchatState) enter(store *sessions.Store, childSessionID, title string, parentScrollOffset int) string {
	if store == nil || childSessionID == "" {
		return "No session store available."
	}
	events, err := store.ReadEvents(childSessionID)
	if err != nil {
		return "Could not load subagent session: " + err.Error()
	}
	s.active = true
	s.childSessionID = childSessionID
	s.childSessionTitle = title
	s.parentScrollOffset = parentScrollOffset
	s.childRows = collapseSpecialistEnvelope(transcriptRowsFromSessionEvents(events))
	return ""
}

// exit deactivates the subchat view and returns the saved parent scroll offset.
func (s *subchatState) exit() int {
	offset := s.parentScrollOffset
	println("DEBUG exit() returning", offset)
	s.active = false
	s.childSessionID = ""
	s.childSessionTitle = ""
	s.parentScrollOffset = 0
	s.childRows = nil
	return offset
}

// renderSubchatNavBar renders the navigation bar shown at the top of the
// subchat view, telling the user how to get back to the main chat.
func renderSubchatNavBar(title string, width int) string {
	nav := "← Back to main chat"
	if title != "" {
		nav += "  ·  " + truncateRunes(title, width-40)
	}
	return runeTheme.accent.Render(nav)
}

func (m model) subchatHeader(width int) string {
	info, ok := m.specialists.getBySessionID(m.subchat.childSessionID)
	if !ok {
		return renderSubchatNavBar(m.subchat.childSessionTitle, width)
	}
	name := strings.TrimSpace(info.name)
	if name == "" {
		name = "specialist"
	}
	task := strings.TrimSpace(info.description)
	if task == "" {
		task = "Task details unavailable"
	}
	status := "Failed"
	if info.status == specialistRunning {
		status = "Running"
	} else if info.status == specialistCompleted {
		status = "Completed"
	}
	var elapsed time.Duration
	if info.status == specialistRunning {
		elapsed = m.now().Sub(info.startedAt)
	} else {
		elapsed = info.completedAt.Sub(info.startedAt)
	}
	lines := []string{
		renderSubchatNavBar("", width),
		"",
		runeTheme.ink.Bold(true).Render(name),
		fitStyledLine(runeTheme.muted.Render(task), width),
		"",
		runeTheme.accent.Render(fmt.Sprintf("%s · %s", status, formatSpecialistElapsed(elapsed))),
	}
	return strings.Join(lines, "\n")
}

func (m model) subchatFooter(width int) string {
	return fitStyledLine(runeTheme.faint.Render("Esc Back · ↑↓ Scroll · Enter Expand"), width)
}

// collapseSpecialistEnvelope hides the raw invocation boilerplate ("# Specialist
// Invocation …", "# Follow-up Instructions …") behind one quiet row, so the
// inspector surfaces name/task/status/tool activity/result — the full text
// stays in the session log.
func collapseSpecialistEnvelope(rows []transcriptRow) []transcriptRow {
	out := make([]transcriptRow, 0, len(rows))
	collapsed := false
	for _, row := range rows {
		if row.kind == rowSystem &&
			(strings.HasPrefix(row.text, "# Specialist Invocation") ||
				strings.HasPrefix(row.text, "# Follow-up Instructions")) {
			if !collapsed {
				out = append(out, transcriptRow{kind: rowSystem, text: "Specialist instruction envelope hidden (full text in session log)."})
				collapsed = true
			}
			continue
		}
		out = append(out, row)
	}
	return out
}
