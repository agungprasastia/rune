package tui

import (
	"context"
	"strings"
	"testing"
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
