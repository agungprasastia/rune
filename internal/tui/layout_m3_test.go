package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"rune/internal/config"
)

func TestM3LayoutUsesMainAndSidebarRects(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.width, m.height, m.altScreen = 160, 50, true
	m.transcript = append(m.transcript, transcriptRow{kind: rowToolCall, tool: "read_file"})
	m.plan.steps = []planStep{{content: "inspect", status: "in_progress"}}

	layout := m.layout()
	if !layout.SidebarVisible() {
		t.Fatal("wide active layout should show sidebar")
	}
	if layout.MainWidth()+layout.Sidebar.width+1 != layout.Width {
		t.Fatalf("layout columns do not fill width: %+v", layout)
	}
	if got := m.chatColumnWidth(); got != layout.MainWidth() {
		t.Fatalf("chat width = %d, want canonical main width %d", got, layout.MainWidth())
	}

	m.sidebarHidden = true
	if m.layout().SidebarVisible() {
		t.Fatal("hidden sidebar should not render")
	}
	if got := m.chatColumnWidth(); got != m.width {
		t.Fatalf("hidden sidebar main width = %d, want %d", got, m.width)
	}
}

func TestM3EmptyComposerUsesBoundedCenteredWidth(t *testing.T) {
	m := newModel(context.Background(), Options{})
	for _, width := range []int{60, 80, 120, 160} {
		m.width, m.height = width, 30
		lines := strings.Split(plainRender(t, m.composerBox(width)), "\n")
		if len(lines) == 0 {
			t.Fatalf("width %d produced no composer", width)
		}
		box := strings.TrimSpace(lines[0])
		if !strings.HasPrefix(box, "╭") || !strings.HasSuffix(box, "╮") {
			t.Fatalf("width %d composer frame = %q", width, lines[0])
		}
		boxWidth := len([]rune(box))
		if boxWidth < 44 && width >= 60 {
			t.Fatalf("width %d composer width = %d, below minimum", width, boxWidth)
		}
		if boxWidth > 72 {
			t.Fatalf("width %d composer width = %d, above maximum", width, boxWidth)
		}
	}
}

func TestM3PresentationStatesStayIndependent(t *testing.T) {
	if DisplayExpanded == DisplayCollapsed {
		t.Fatal("display states must be distinct")
	}
	if ExecutionRunning == ExecutionFailed {
		t.Fatal("execution states must be distinct")
	}
	if (model{}).transcriptScrollState() != FollowTail {
		t.Fatal("zero scroll offset should follow tail")
	}
	if (model{chatScrollOffset: 1}).transcriptScrollState() != UserAnchored {
		t.Fatal("positive scroll offset should anchor user")
	}
	row := transcriptRow{kind: rowToolCall}
	if row.executionState() != ExecutionRunning || row.displayState() != DisplayCollapsed {
		t.Fatalf("tool execution/display states = %q/%q", row.executionState(), row.displayState())
	}
	row.expanded = true
	if row.executionState() != ExecutionRunning || row.displayState() != DisplayExpanded {
		t.Fatalf("expanded tool states = %q/%q", row.executionState(), row.displayState())
	}
}

func TestM31SidebarUsesMainAndSidebarConstraints(t *testing.T) {
	m := newModel(context.Background(), Options{AltScreen: true})
	m.height = 30
	m.transcript = appendRow(m.transcript, rowAssistant, "conversation")
	for _, width := range []int{60, 80, 100, 120, 160, 200} {
		m.width = width
		layout := m.layout()
		if layout.SidebarVisible() {
			if layout.Sidebar.width < sidebarMinWidth || layout.Sidebar.width > sidebarMaxWidth {
				t.Fatalf("width %d sidebar = %#v, outside constraints", width, layout.Sidebar)
			}
			if layout.Main.width < sidebarMinMainWidth {
				t.Fatalf("width %d main = %#v, below minimum", width, layout.Main)
			}
			if layout.Main.width+layout.Sidebar.width+1 != width {
				t.Fatalf("width %d columns do not fill shell: %#v", width, layout)
			}
		}
	}
}

func TestM31SidebarHidesEmptyOptionalSections(t *testing.T) {
	m := newModel(context.Background(), Options{AltScreen: true})
	m.width, m.height = 160, 30
	m.transcript = appendRow(m.transcript, rowAssistant, "conversation")
	plain := strings.Join(m.renderContextSidebar(sidebarWidth(m.width), m.height), "\n")
	for _, filler := range []string{"no agents spawned", "no active plan", "no files touched"} {
		if strings.Contains(plain, filler) {
			t.Fatalf("empty sidebar contains filler %q:\n%s", filler, plain)
		}
	}
}

func TestM31OverlayRectUsesShellBody(t *testing.T) {
	m := newModel(context.Background(), Options{AltScreen: true})
	m.width, m.height = 160, 40
	frame := m.scrollableTranscriptFrame(m.pinnedTitleBar(m.chatColumnWidth()), m.footerView(m.chatColumnWidth()))
	rect := frame.OverlayRect(7)
	if !frame.bodyRect.contains(rect.x, rect.y) || !frame.bodyRect.contains(rect.x, rect.y+rect.height-1) {
		t.Fatalf("overlay rect %#v outside shell body %#v", rect, frame.bodyRect)
	}
	if rect.y != frame.bodyRect.y+(frame.bodyRect.height-rect.height)/2 {
		t.Fatalf("overlay y=%d, want centered in body %#v", rect.y, frame.bodyRect)
	}
}

func TestM31ComposedShellRowsMeetColumnSeam(t *testing.T) {
	m := newModel(context.Background(), Options{AltScreen: true, ProviderName: "openai", ModelName: "gpt-4.1"})
	m.width, m.height = 160, 50
	m.transcript = appendRow(m.transcript, rowAssistant, strings.Repeat("readable transcript content ", 8))
	m.activeSession.Title = "Indonesian Greeting Exchange"
	m.mcpConfig.Servers = map[string]config.MCPServerConfig{"exa": {}}

	layout := m.layout()
	if !layout.SidebarVisible() {
		t.Fatal("expected sidebar for wide shell")
	}
	view := plainRender(t, m.View())
	want := layout.Main.width + 1 + layout.Sidebar.width
	for index, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got != want {
			t.Fatalf("row %d width = %d, want shell width %d: %q", index, got, want, line)
		}
	}
}
