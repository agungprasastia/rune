package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"image/color"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rune/internal/agent"
	"rune/internal/tools"
)

// M3.3 smoke matrix: every structural/visual invariant is asserted across the
// five reference terminal sizes so geometry cannot drift on one tier only.
var m33Sizes = []struct {
	name         string
	width        int
	height       int
	sidebarWidth int // 0 when the constraints hide the sidebar at this size
}{
	{"60x20", 60, 20, 0},
	{"80x24", 80, 24, 0},
	{"100x30", 100, 30, 0}, // main would be 71 < sidebarMinMainWidth
	{"120x40", 120, 40, 28},
	{"160x50", 160, 50, 38},
}

func m33NewConversation(t *testing.T, width, height int) model {
	t.Helper()
	m := newModel(context.Background(), Options{AltScreen: true})
	m.width, m.height = width, height
	m.headerPrinted = true
	m.transcript = appendRow(m.transcript, rowUser, "inspect the failing test")
	return m
}

func m33AssertFits(t *testing.T, name string, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	for index, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("%s: line %d is %d cells wide (max %d): %q", name, index, got, width, line)
		}
	}
	if len(lines) != height {
		t.Fatalf("%s: view height = %d, want %d", name, len(lines), height)
	}
}

var m33BuildWord = regexp.MustCompile(`(?i)\bbuild\b`)

func TestM33NoBuildModeAnywhere(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			states := map[string]model{}
			empty := newModel(context.Background(), Options{})
			empty.width, empty.height = tc.width, tc.height
			empty.altScreen = true
			states["empty"] = empty

			convo := m33NewConversation(t, tc.width, tc.height)
			convo.transcript = appendRow(convo.transcript, rowAssistant, "Done — the fix is in.")
			convo.flushed = len(convo.transcript)
			states["conversation"] = convo

			help := m33NewConversation(t, tc.width, tc.height)
			help.helpOverlay = true
			states["help-overlay"] = help

			for stateName, m := range states {
				view := plainRender(t, m.View())
				if hit := m33BuildWord.FindString(view); hit != "" {
					t.Fatalf("%s/%s: view must not surface a Build mode (found %q)", tc.name, stateName, hit)
				}
				label, _ := m.modeLabel()
				if m33BuildWord.MatchString(label) {
					t.Fatalf("mode label %q must not be Build", label)
				}
			}
		})
	}
}

func TestM33ViewGeometryAtAllSizes(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name+"/empty", func(t *testing.T) {
			m := newModel(context.Background(), Options{})
			m.width, m.height = tc.width, tc.height
			m.altScreen = true
			m33AssertFits(t, tc.name, plainRender(t, m.View()), tc.width, tc.height)
		})
		t.Run(tc.name+"/conversation", func(t *testing.T) {
			m := m33NewConversation(t, tc.width, tc.height)
			m33AssertFits(t, tc.name, plainRender(t, m.View()), tc.width, tc.height)
		})
	}
}

func TestM33EmptyStateIsOneCenteredCluster(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(context.Background(), Options{})
			m.width, m.height = tc.width, tc.height
			m.altScreen = true
			view := plainRender(t, m.View())
			lines := strings.Split(view, "\n")

			taglineY, composerTopY := -1, -1
			for index, line := range lines {
				switch {
				case taglineY < 0 && strings.Contains(line, emptyStateTagline):
					taglineY = index
				case composerTopY < 0 && index > len(lines)/3 && strings.HasPrefix(strings.TrimSpace(line), "╭"):
					composerTopY = index
				}
			}
			if taglineY < 0 || composerTopY < 0 {
				t.Fatalf("cluster missing tagline/composer (tagline=%d composer=%d):\n%s", taglineY, composerTopY, view)
			}
			// Wordmark, tagline, and composer share ONE cluster: the gap between
			// the brand block and the input stays small instead of pushing the
			// composer to a distant footer.
			if gap := composerTopY - taglineY; gap < 1 || gap > 4 {
				t.Fatalf("composer sits %d rows below the tagline, want 1..4:\n%s", gap, view)
			}
			if !strings.Contains(view, composerPlaceholder) {
				t.Fatalf("cluster composer missing placeholder:\n%s", view)
			}
			if !strings.Contains(view, "Tab mode") || !strings.Contains(view, "Tip: use / for commands") {
				t.Fatalf("cluster hints missing:\n%s", view)
			}
			// The footer keeps only the status line — no second composer.
			footer := strings.Join(lines[len(lines)-2:], "\n")
			if strings.Contains(footer, "describe a task") || strings.Contains(footer, "╭") {
				t.Fatalf("footer must stay composer-free in cluster mode:\n%s", footer)
			}

			if tc.width >= 80 {
				if !strings.Contains(view, "█") {
					t.Fatalf("wide terminal should render the full wordmark:\n%s", view)
				}
			} else if !strings.Contains(view, "R U N E") {
				t.Fatalf("narrow terminal should use the compact mark:\n%s", view)
			}
		})
	}
}

func TestM33FirstPromptMovesComposerToFooter(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(context.Background(), Options{})
			m.width, m.height = tc.width, tc.height
			m.altScreen = true
			m.input.SetValue("inspect the repo")

			updated, _ := m.Update(testKey(tea.KeyEnter))
			next := updated.(model)
			next.width, next.height = tc.width, tc.height

			view := plainRender(t, next.View())
			if strings.Contains(view, emptyStateTagline) {
				t.Fatalf("%s: cluster should collapse after the first prompt", tc.name)
			}
			if !strings.Contains(view, composerPlaceholder) {
				t.Fatalf("%s: docked composer placeholder missing after prompt:\n%s", tc.name, view)
			}
			// The docked composer lives in the footer band (bottom quarter).
			lines := strings.Split(view, "\n")
			composerY := -1
			for index, line := range lines {
				if strings.Contains(line, composerPlaceholder) {
					composerY = index
				}
			}
			if composerY < (len(lines)*3)/4 {
				t.Fatalf("%s: composer at row %d/%d, want docked near the bottom", tc.name, composerY, len(lines))
			}
		})
	}
}

func TestM33TabCyclesAskPlanAuto(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(context.Background(), Options{PermissionMode: agent.PermissionModeAsk})
			m.width, m.height = tc.width, tc.height

			want := []agent.PermissionMode{
				agent.PermissionModePlan,
				agent.PermissionModeAuto,
				agent.PermissionModeAsk,
			}
			for _, next := range want {
				updated, _ := m.Update(testKey(tea.KeyTab))
				m = updated.(model)
				if m.permissionMode != next {
					t.Fatalf("Tab from previous mode: got %q, want %q", m.permissionMode, next)
				}
			}
			// Shift+Tab steps back: Ask → Auto.
			updated, _ := m.Update(testKeyShift(tea.KeyTab))
			m = updated.(model)
			if m.permissionMode != agent.PermissionModeAuto {
				t.Fatalf("Shift+Tab from Ask: got %q, want auto", m.permissionMode)
			}

			// Composer metadata tracks the ring with title-case labels.
			if metadata := plainRender(t, m.composerMetadataLine(80)); !strings.Contains(metadata, "Auto") {
				t.Fatalf("metadata = %q, want the Auto label", metadata)
			}
		})
	}
}

func TestM33TabRespectsAutocompleteAndModals(t *testing.T) {
	m := newModel(context.Background(), Options{PermissionMode: agent.PermissionModeAsk})
	m.width, m.height = 100, 30

	// Autocomplete contract first: Tab moves the suggestion cursor.
	m = typeRunes(t, m, "/")
	if !m.suggestionsActive() {
		t.Fatal("precondition: / should open command suggestions")
	}
	idx := m.suggestionIdx
	updated, _ := m.Update(testKey(tea.KeyTab))
	m = updated.(model)
	if m.permissionMode != agent.PermissionModeAsk {
		t.Fatalf("Tab while suggestions open cycled mode to %q", m.permissionMode)
	}
	if m.suggestionIdx == idx && len(m.suggestions) > 1 {
		t.Fatalf("Tab should move the suggestion cursor")
	}

	// A modal owns Tab too: the help overlay swallows it.
	m.helpOverlay = true
	updated, _ = m.Update(testKey(tea.KeyTab))
	m = updated.(model)
	if m.permissionMode != agent.PermissionModeAsk {
		t.Fatalf("Tab under help overlay cycled mode to %q", m.permissionMode)
	}
}

func TestM33SidebarConstraintsAndStability(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := m33NewConversation(t, tc.width, tc.height)
			layout := m.layout()
			if got := layout.Sidebar.width; got != tc.sidebarWidth {
				t.Fatalf("sidebar width = %d, want %d (total %d)", got, tc.sidebarWidth, tc.width)
			}
			if layout.SidebarVisible() {
				if layout.Sidebar.width < sidebarMinWidth || layout.Sidebar.width > sidebarMaxWidth {
					t.Fatalf("sidebar width %d outside [%d,%d]", layout.Sidebar.width, sidebarMinWidth, sidebarMaxWidth)
				}
				if layout.Sidebar.width*4 > tc.width {
					t.Fatalf("sidebar %d exceeds ~25%% of %d", layout.Sidebar.width, tc.width)
				}
			}
			// Stability: content changes never resize the columns (no re-wrap).
			before := m.layout()
			withPlan := runningPlanModel(t, 3)
			withPlan.width, withPlan.height = tc.width, tc.height
			withPlan.altScreen = true
			withPlan.transcript = appendRow(withPlan.transcript, rowUser, "inspect")
			after := withPlan.layout()
			if before.Main.width != after.Main.width || before.Sidebar.width != after.Sidebar.width {
				t.Fatalf("columns moved when plan appeared: %v -> %v", before, after)
			}
			// Narrow tier: full-width main, hidden sidebar preserved as usable area.
			if !layout.SidebarVisible() && layout.Main.width != tc.width {
				t.Fatalf("hidden sidebar must yield full main width, got %d", layout.Main.width)
			}
		})
	}
}

func TestM33ProseReadableToolsFullWidth(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := m33NewConversation(t, tc.width, tc.height)
			long := strings.Repeat("prose words fill the readable column ", 8)
			m.transcript = appendRow(m.transcript, rowAssistant, long)
			m.flushed = len(m.transcript)

			row := transcriptRow{kind: rowAssistant, text: long, final: true}
			rendered := plainRender(t, m.renderRow(row, tc.width, buildRowContext(nil)))
			for _, line := range strings.Split(rendered, "\n") {
				if w := lipgloss.Width(strings.TrimRight(line, " ")); w > assistantMeasureCap {
					t.Fatalf("prose line %d exceeds readable cap %d: %q", w, assistantMeasureCap, line)
				}
			}
			m33AssertFits(t, tc.name, plainRender(t, m.View()), tc.width, tc.height)
		})
	}
}

func TestM33ToolCollapseExpandWithKeyboard(t *testing.T) {
	m := m33NewConversation(t, 100, 30)
	detail := strings.Repeat("output line\n", cardBodyMaxLines+6)
	m.transcript = appendTranscriptRow(m.transcript, transcriptRow{
		kind: rowToolResult, id: "t1", tool: "bash", status: tools.StatusOK, detail: detail,
	})

	view := plainRender(t, m.View())
	want := fmt.Sprintf("▸ %d lines — Enter · click to expand", cardBodyMaxLines+6)
	if !strings.Contains(view, want) {
		t.Fatalf("collapsed tool footer %q missing:\n%s", want, view)
	}

	rowIndex := len(m.transcript) - 1
	m.hover = hoverTarget{kind: hoverTranscript, toggleRow: rowIndex}
	updated, _ := m.Update(testKey(tea.KeyEnter))
	m = updated.(model)
	if !m.transcript[rowIndex].expanded {
		t.Fatal("Enter should expand the hovered collapsed tool card")
	}
	if strings.Contains(plainRender(t, m.View()), "click to expand") {
		t.Fatal("expanded card still advertises collapse hint")
	}
}

func TestM33SubagentIsReadOnlyInspector(t *testing.T) {
	for _, tc := range m33Sizes {
		t.Run(tc.name, func(t *testing.T) {
			m := m33NewConversation(t, tc.width, tc.height)
			m.subchat.active = true
			m.subchat.childSessionID = "child-1"
			m.subchat.childSessionTitle = "explorer · map runtime"
			m.subchat.parentScrollOffset = 7
			envelope := "# Specialist Invocation\n\nSpecialist: explorer\n\n## Specialist Instructions\n\nmap everything\n\n## Reporting\n\n- report back"
			m.subchat.childRows = appendRow(nil, rowSystem, envelope)
			m.subchat.childRows = appendRow(m.subchat.childRows, rowToolCall, "tool call: read_file")
			m.subchat.childRows = appendTranscriptRow(m.subchat.childRows, transcriptRow{
				kind: rowToolResult, id: "c1", tool: "read_file", status: tools.StatusOK,
				detail: strings.Repeat("child line\n", cardBodyMaxLines+4),
			})
			m.subchat.childRows = appendRow(m.subchat.childRows, rowAssistant, "Runtime flows from cmd/rune.")
			m.subchat.childRows = collapseSpecialistEnvelope(m.subchat.childRows)

			view := plainRender(t, m.View())
			m33AssertFits(t, tc.name, view, tc.width, tc.height)
			if strings.Contains(view, composerPlaceholder) {
				t.Fatalf("%s: inspector must not render the composer", tc.name)
			}
			if !strings.Contains(view, "Esc Back · ↑↓ Scroll · Enter Expand") {
				t.Fatalf("%s: inspector footer text wrong:\n%s", tc.name, view)
			}
			if !strings.Contains(view, "instruction envelope hidden") || strings.Contains(view, "# Specialist Invocation") {
				t.Fatalf("%s: specialist boilerplate not collapsed", tc.name)
			}

			// Hidden input paths stay dead.
			updated, _ := m.Update(testKey('x'))
			m = updated.(model)
			updated, _ = m.Update(tea.PasteMsg{Content: "pasted"})
			m = updated.(model)
			if m.composerValue() != "" {
				t.Fatalf("%s: inspector leaked input into composer %q", tc.name, m.composerValue())
			}

			for i := 0; i < 40; i++ {
				m.subchat.childRows = appendRow(m.subchat.childRows, rowAssistant, fmt.Sprintf("filler row %d", i))
			}

			// Scrolling works and ArrowUp no longer exits.
			offset := m.chatScrollOffset
			updated, _ = m.Update(testKey(tea.KeyUp))
			m = updated.(model)
			if !m.subchat.active {
				t.Fatalf("%s: ArrowUp must scroll, not exit", tc.name)
			}
			if m.chatScrollOffset == offset {
				t.Fatalf("%s: ArrowUp did not scroll (offset %d)", tc.name, offset)
			}

			// Enter expands the hovered child tool card in place.
			resultIndex := 2
			m.hover = hoverTarget{kind: hoverTranscript, toggleRow: resultIndex}
			updated, _ = m.Update(testKey(tea.KeyEnter))
			m = updated.(model)
			if !m.subchat.childRows[resultIndex].expanded {
				t.Fatalf("%s: Enter did not expand child tool card", tc.name)
			}
			if len(m.transcript) > 0 && m.transcript[len(m.transcript)-1].expanded {
				t.Fatalf("%s: parent transcript mutated by subchat expand", tc.name)
			}
			m.hover = hoverTarget{}

			// Resize keeps the inspector coherent.
			w2, h2 := tc.width-10, tc.height-4
			updated, _ = m.Update(tea.WindowSizeMsg{Width: w2, Height: h2})
			m = updated.(model)
			if !m.subchat.active {
				t.Fatalf("%s: resize dropped the inspector", tc.name)
			}
			m33AssertFits(t, tc.name+"/resize", plainRender(t, m.View()), w2, h2)

			// Esc returns to the parent at its saved scroll offset.
			updated, _ = m.Update(testKey(tea.KeyEsc))
			m = updated.(model)
			if m.subchat.active {
				t.Fatalf("%s: Esc must exit the inspector", tc.name)
			}
			// The saved parent offset must survive as a value that is still valid
			// against the parent viewport (sync re-clamps across domains).
			_, parentMax := m.chatScrollMetrics()
			if m.chatScrollOffset < 0 || m.chatScrollOffset > parentMax {
				t.Fatalf("%s: parent scroll offset %d outside [0,%d]", tc.name, m.chatScrollOffset, parentMax)
			}
		})
	}
}

func TestM33CanvasRepaintOnlyOnEvents(t *testing.T) {
	defer applyTheme(themeDark, true)
	applyTheme(themeDark, true)
	m := m33NewConversation(t, 100, 30)

	first := m.View().BackgroundColor
	second := m.View().BackgroundColor
	if first == nil || second == nil {
		t.Fatal("dark theme must paint the canvas color")
	}
	if first != second {
		t.Fatalf("steady-state frames changed background identity: %#v vs %#v", first, second)
	}

	bumped := m
	bumped.terminalFocused = false
	updated, _ := bumped.Update(tea.FocusMsg{})
	focused := updated.(model)
	third := focused.View().BackgroundColor
	if third == nil || third == first {
		t.Fatalf("focus/resume must re-emit the canvas once, got identity %#v", third)
	}

	// Honest detection: a light probe under a dark named theme keeps painting;
	// the flag itself records reality.
	lightProbe := tea.BackgroundColorMsg{Color: lipgloss.Color("#ffffff")}
	updated, _ = m.Update(lightProbe)
	light := updated.(model)
	if light.hasDarkBg {
		t.Fatal("light probe must be recorded, not forced dark")
	}
}

func TestM33LayeredSurfacesSubtle(t *testing.T) {
	defer applyTheme(themeDark, true)
	applyTheme(themeDark, true)
	canvas, panel, raised := runeTheme.bgCanvas, runeTheme.bgPanel, runeTheme.bgPrompt
	eight := func(c color.Color) (uint32, uint32, uint32) {
		r, g, b, _ := c.RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	cr, cg, cb := eight(canvas)
	pr, pg, pb := eight(panel)
	rr, rg, rb := eight(raised)
	dist := func(aR, aG, aB uint32, bR, bG, bB uint32) uint32 {
		dr := int(aR) - int(bR)
		dg := int(aG) - int(bG)
		db := int(aB) - int(bB)
		if dr < 0 {
			dr = -dr
		}
		if dg < 0 {
			dg = -dg
		}
		if db < 0 {
			db = -db
		}
		return uint32(dr + dg + db)
	}
	// Layers step AWAY from the canvas in one direction with small deltas:
	// subtle stacking, not giant gray slabs.
	if d := dist(pr, pg, pb, cr, cg, cb); d == 0 || d > 40 {
		t.Fatalf("surface/canvas delta %d outside subtle range", d)
	}
	if d := dist(rr, rg, rb, pr, pg, pb); d == 0 || d > 40 {
		t.Fatalf("raised/surface delta %d outside subtle range", d)
	}
	if dist(rr, rg, rb, pr, pg, pb) <= dist(pr, pg, pb, cr, cg, cb) {
		t.Fatal("raised surface must step further from the canvas than the base surface")
	}
}
