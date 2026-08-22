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

// m33ComposerGeometry reads the composer box geometry straight out of a
// rendered view: box width (border corners inclusive), left pad column, and
// the metadata row embedded just above the bottom rule.
func m33ComposerGeometry(t *testing.T, view string) (boxWidth, leftPad, metaLead, metaVisible int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	topRow := -1
	for index, raw := range lines {
		runes := []rune(ansiPattern.ReplaceAllString(raw, ""))
		start, end := -1, -1
		for pos, glyph := range runes {
			switch glyph {
			case '╭':
				if start < 0 {
					start = pos
				}
			case '╮':
				end = pos
			}
		}
		if start < 0 || end < start {
			continue
		}
		boxWidth = end - start + 1
		leftPad = start
		topRow = index
		break
	}
	if topRow < 0 {
		t.Fatal("composer top border row not found in view")
	}
	for index := topRow + 1; index < len(lines); index++ {
		if !strings.Contains(plainRender(t, lines[index]), "╰") {
			continue
		}
		// Metadata is the last row INSIDE the box, directly above the rule.
		metaPlain := plainRender(t, lines[index-1])
		metaLead = len(metaPlain) - len(strings.TrimLeft(metaPlain, " "))
		metaVisible = lipgloss.Width(strings.TrimRight(metaPlain, " ")) - metaLead
		return boxWidth, leftPad, metaLead, metaVisible
	}
	t.Fatal("composer metadata row not found after the bottom rule")
	return 0, 0, 0, 0
}

func TestM33StartupComposerStableGeometry(t *testing.T) {
	longInput := strings.Repeat("wrap this long sentence ", 12)
	tabTimes := func(count int) func(model) model {
		return func(m model) model {
			for i := 0; i < count; i++ {
				next, _ := m.Update(testKey(tea.KeyTab))
				m = next.(model)
			}
			return m
		}
	}
	scenarios := []struct {
		name   string
		mutate func(model) model
	}{
		{"empty", func(m model) model { return m }},
		{"one-char", func(m model) model { return typeRunes(t, m, "h") }},
		{"sentence", func(m model) model { return typeRunes(t, m, "fix the failing test in ./pkg") }},
		{"long-wrapped", func(m model) model { return typeRunes(t, m, longInput) }},
		{"multiline", func(m model) model {
			text := "one\ntwo\nthree\nfour"
			next := m
			next.setComposerState(composerState{text: text, cursor: len([]rune(text))})
			return next
		}},
		{"tab-plan", tabTimes(1)},
		{"tab-auto", tabTimes(2)},
		{"tab-back-ask", tabTimes(3)},
		{"copy-status", func(m model) model { next := m; next.copyStatus = "Copied transcript"; return next }},
		{"attachment", func(m model) model { next := m; next.pendingImageLabels = []string{"shot.png"}; return next }},
	}
	for _, tc := range []struct {
		name   string
		width  int
		height int
	}{{"60x20", 60, 20}, {"80x24", 80, 24}, {"120x40", 120, 40}, {"160x50", 160, 50}} {
		t.Run(tc.name, func(t *testing.T) {
			base := newModel(context.Background(), Options{})
			base.width, base.height = tc.width, tc.height
			base.altScreen = true
			wantBox, wantLeft, _, _ := m33ComposerGeometry(t, plainRender(t, base.View()))
			wantBoxExpected := minInt(clampInt(tc.width*45/100, 44, 72), tc.width)
			if wantBox != wantBoxExpected {
				t.Fatalf("startup box width = %d, want %d", wantBox, wantBoxExpected)
			}
			for _, sc := range scenarios {
				m := sc.mutate(base)
				gotBox, gotLeft, metaLead, metaVisible := m33ComposerGeometry(t, plainRender(t, m.View()))
				if gotBox != wantBox || gotLeft != wantLeft {
					t.Fatalf("%s: box geometry changed to (%d,+%d), want (%d,+%d)", sc.name, gotBox, gotLeft, wantBox, wantLeft)
				}
				// Metadata is a boxed row: it must start exactly at the composer's
				// left border column.
				if metaLead != wantLeft {
					t.Fatalf("%s: metadata starts at column %d, want %d (box-aligned)", sc.name, metaLead, wantLeft)
				}
				// The metadata row is part of the SAME box: its rendered width
				// equals the composer width, never the full main width.
				if metaVisible != wantBox {
					t.Fatalf("%s: metadata row width %d != composer width %d", sc.name, metaVisible, wantBox)
				}
				m33AssertFits(t, tc.name+"/"+sc.name, plainRender(t, m.View()), tc.width, tc.height)
			}
		})
	}
}

func TestM33StartupComposerResizeRecalc(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.width, m.height = 120, 40
	m.altScreen = true

	boxAt := func(width, height int) int {
		next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
		m = next.(model)
		box, _, _, _ := m33ComposerGeometry(t, plainRender(t, m.View()))
		return box
	}
	if got := boxAt(120, 40); got != 54 {
		t.Fatalf("box at 120 cols = %d, want 54", got)
	}
	if got := boxAt(160, 50); got != 72 {
		t.Fatalf("box at 160 cols = %d, want 72", got)
	}
	if got := boxAt(80, 24); got != 44 {
		t.Fatalf("box at 80 cols = %d, want min 44", got)
	}
}

func TestM33FirstSubmitMovesToActiveLayout(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.width, m.height = 120, 40
	m.altScreen = true

	box, leftPad, _, _ := m33ComposerGeometry(t, plainRender(t, m.View()))
	if box != 54 || leftPad == 0 {
		t.Fatalf("pre-submit startup geometry: box=%d left=%d, want compact centered", box, leftPad)
	}

	// First turn starts: the user prompt lands and the layout flips to the
	// full-main active composer. No intermediate wide frame may exist.
	m.transcript = appendRow(m.transcript, rowUser, "fix it")

	box, leftPad, _, _ = m33ComposerGeometry(t, plainRender(t, m.View()))
	if box <= 54 || leftPad != 0 {
		t.Fatalf("post-submit geometry: box=%d left=%d, want full-width docked", box, leftPad)
	}
	if m.transcriptEmpty() {
		t.Fatal("conversation with a user row must not count as empty")
	}
	view := plainRender(t, m.View())
	if strings.Contains(view, emptyStateTagline) {
		t.Fatal("startup cluster must disappear once the conversation starts")
	}
}

func TestM33MinimalChrome(t *testing.T) {
	m := newModel(context.Background(), Options{ModelName: "gpt-4o", ProviderName: "openai"})
	m.width, m.height = 120, 40
	m.altScreen = true
	m.gitBranch = "main"
	m.cwd = `D:\proj\demo`
	m.appVersion = "dev"
	m.transcript = appendRow(m.transcript, rowUser, "hello")
	m.transcript = appendRow(m.transcript, rowAssistant, "hi there")

	view := plainRender(t, m.View())
	lines := strings.Split(view, "\n")

	for _, banned := range []string{"[?]help", "[Enter]send", "[Ctrl+X]cmds", "[Shift+Tab]mode", "/ commands"} {
		if strings.Contains(view, banned) {
			t.Fatalf("persistent chrome %q must be gone from the conversation view", banned)
		}
	}
	// Transcript-first: no workspace header row above the content and no blank
	// placeholder left behind.
	if strings.Contains(view, `D:\proj\demo`) && strings.Index(view, `D:\proj\demo`) < strings.Index(view, "hello") {
		t.Fatalf("workspace path must not render as a main-view header:\n%s", firstLines(lines, 3))
	}

	// The one-line spacer sits between the transcript region and the box:
	// the footer's FIRST line is blank, the next non-blank is the composer.
	footerLines := viewLines(m.footerView(m.chatColumnWidth()))
	if len(footerLines) < 2 || strings.TrimSpace(footerLines[0]) != "" {
		t.Fatalf("expected a breathing-room spacer row before the composer, footer=%q", strings.Join(footerLines[:minInt(3, len(footerLines))], "|"))
	}

	// Project identity anchors the SIDEBAR foot, muted, no card.
	if !strings.Contains(view, `D:\proj\demo`) || !strings.Contains(view, "main · Rune dev") {
		t.Fatalf("sidebar must carry project path and branch·version at its foot:\n%s", view)
	}
}

func firstLines(lines []string, count int) string {
	end := minInt(count, len(lines))
	return strings.Join(lines[:end], "\n")
}

func TestM33BlockingModalHidesComposer(t *testing.T) {
	m := newModel(context.Background(), Options{ModelName: "gpt-4o", ProviderName: "openai"})
	m.width, m.height = 120, 40
	m.altScreen = true
	m.transcript = appendRow(m.transcript, rowUser, "hello")

	if before := plainRender(t, m.View()); !strings.Contains(before, "╭") {
		t.Fatal("precondition: composer visible without a modal")
	}

	m.picker = &commandPicker{}
	over := plainRender(t, m.View())
	// The picker draws its OWN box and model list; the composer is identified
	// by its placeholder, which must be gone while the modal owns input.
	if strings.Contains(over, "describe a task for rune") {
		t.Fatalf("blocking picker must hide the composer (placeholder leaked):\n%s", over)
	}

	m.picker = nil
	restored := plainRender(t, m.View())
	if !strings.Contains(restored, "describe a task for rune") || !strings.Contains(restored, "gpt-4o") {
		t.Fatalf("composer must return after the modal closes:\n%s", restored)
	}
}
