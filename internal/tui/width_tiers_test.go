package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"rune/internal/agent"
	"rune/internal/sandbox"
	"rune/internal/tools"
)

// The spec's adaptive acceptance criteria: a table across widths asserting
// which segments render at each tier.
func TestWidthTierSegments(t *testing.T) {
	diff := strings.Join([]string{
		"+++ b/internal/cli/root.go",
		"@@ -1,2 +1,2 @@",
		" package cli",
		"-var old = 1",
		"+var new = 2",
	}, "\n")

	cases := []struct {
		width int

		wantArg    bool // tool-card arg column
		wantGutter bool // diff line-number gutter
	}{
		{width: 58, wantArg: false, wantGutter: false},
		{width: 70, wantArg: false, wantGutter: false},
		{width: 80, wantArg: false, wantGutter: true},
		{width: 100, wantArg: true, wantGutter: true},
		{width: 120, wantArg: true, wantGutter: true},
	}

	for _, tc := range cases {
		m := newModel(context.Background(), Options{
			Cwd:          "/Users/dev/rune-project-workspace",
			ProviderName: "anthropic",
			ModelName:    "claude-sonnet-4.5",
		})
		m.width, m.height = tc.width, 30

		title := plainRender(t, m.titleBar(tc.width))
		if strings.Contains(title, "200K") || strings.Contains(title, "claude-sonnet-4.5") {
			t.Errorf("width %d: title must stay minimal (%q)", tc.width, title)
		}

		rows := []transcriptRow{
			{kind: rowToolCall, id: "c", tool: "custom_tool", detail: "internal/cli", arg: "RegisterFlag"},
			{kind: rowToolResult, id: "c", tool: "custom_tool", status: tools.StatusOK, detail: "ok"},
		}
		rc := buildRowContext(rows)
		card := plainRender(t, m.renderRow(rows[1], tc.width, rc))
		if got := strings.Contains(card, "RegisterFlag"); got != tc.wantArg {
			t.Errorf("width %d: card arg column = %v, want %v (%q)", tc.width, got, tc.wantArg, card)
		}

		diffRow := transcriptRow{kind: rowToolResult, id: "d", tool: "edit_file", status: tools.StatusOK, detail: diff}
		diffCard := plainRender(t, m.renderRow(diffRow, tc.width, buildRowContext(nil)))
		if got := strings.Contains(diffCard, "   2 + var new = 2") || strings.Contains(diffCard, "   2 +"); got != tc.wantGutter {
			t.Errorf("width %d: diff gutter = %v, want %v (%q)", tc.width, got, tc.wantGutter, diffCard)
		}

		status := plainRender(t, m.statusLine(tc.width))
		// M3.3: mode lives in the composer metadata; the steady status line stays
		// free of model/provider.
		if strings.Contains(status, "interactive") || strings.Contains(status, "claude-sonnet-4.5") || strings.Contains(status, "anthropic") {
			t.Errorf("width %d: status should not include surface, model, or provider (%q)", tc.width, status)
		}
		metadata := plainRender(t, m.composerMetadataLine(tc.width))
		if !strings.Contains(metadata, "Auto") || !strings.Contains(metadata, "claude-sonnet-4.5") {
			t.Errorf("width %d: composer metadata = %q, want mode and model", tc.width, metadata)
		}
	}
}

func TestTinyTierSingleSegmentAndRailLessCards(t *testing.T) {
	m := newModel(context.Background(), Options{ProviderName: "anthropic", ModelName: "claude-sonnet-4.5"})
	m.width, m.height = 40, 20

	status := plainRender(t, m.statusLine(40))
	// M3.3 tiny tier: no mode chip (composer metadata owns it), no model text.
	if strings.Contains(status, "anthropic") || strings.Contains(status, "claude-sonnet-4.5") || strings.Contains(status, "Auto") {
		t.Fatalf("tiny status = %q, want it quiet (no mode/model/provider)", status)
	}
	metadata := plainRender(t, m.composerMetadataLine(40))
	if !strings.Contains(metadata, "Auto") || !strings.Contains(metadata, "claude-sonnet-4.5") {
		t.Fatalf("tiny composer metadata = %q, want mode and model", metadata)
	}

	row := transcriptRow{kind: rowToolResult, id: "c", tool: "grep", status: tools.StatusOK, detail: "a.go:1: x"}
	card := plainRender(t, m.renderRow(row, 40, buildRowContext(nil)))
	for _, line := range strings.Split(card, "\n") {
		if strings.HasPrefix(line, "│") || strings.HasSuffix(line, "│") {
			t.Fatalf("tiny card keeps side borders: %q", line)
		}
	}
}

func TestTitleBarKeepsWorkspaceWithLongBranch(t *testing.T) {
	m := newModel(context.Background(), Options{
		Cwd:          "/workspace/rune",
		ProviderName: "ollama-cloud",
		ModelName:    "cogito-2.1:671b-extra-long-model-name",
	})
	m.gitBranch = "feat/tui-assistant-response-cleanup"

	got := plainRender(t, m.titleBar(108))
	for _, want := range []string{"/workspace/rune", "feat/tui-assistant-response-cleanup"} {
		if !strings.Contains(got, want) {
			t.Fatalf("title bar = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "cogito-2.1") {
		t.Fatalf("title bar = %q, should not show model (moved to composer metadata)", got)
	}
	for index, line := range strings.Split(got, "\n") {
		if width := lipgloss.Width(line); width > 108 {
			t.Fatalf("title line %d width = %d, want <= 108: %q", index, width, line)
		}
	}
}

// The spec's hard rendering invariant: never emit a styled line wider than
// the terminal, across the whole frame at every tier — including the empty
// state, ask-user rows, permission details, and pending image chips, which
// each overflowed at some width before being fitted.
func TestViewNeverExceedsTerminalWidth(t *testing.T) {
	diff := "+++ b/a.go\n@@ -1,1 +1,1 @@\n-old line that is reasonably long for the card\n+new line that is reasonably long for the card"
	for _, width := range []int{24, 40, 58, 70, 80, 100, 120} {
		m := newModel(context.Background(), Options{
			Cwd:          "/Users/dev/rune-project-workspace",
			ProviderName: "anthropic",
			ModelName:    "claude-sonnet-4.5",
		})
		m.width, m.height = width, 24

		// Empty state first: the centered tagline/hint must also fit.
		for index, line := range strings.Split(viewString(m.View()), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d: empty-state line %d is %d cells wide: %q", width, index, got, line)
			}
		}

		m.pendingImageLabels = []string{"Screenshot 2026-06-10 at 09.41.13.png", "Screenshot 2026-06-10 at 09.44.02.png"}
		m.transcript = append(m.transcript,
			transcriptRow{kind: rowUser, text: "please change the longest line in the file to something even longer than before"},
			transcriptRow{kind: rowToolCall, id: "c1", tool: "grep", detail: "internal/cli", arg: "RegisterFlag|flag\\."},
			transcriptRow{kind: rowToolResult, id: "c1", tool: "grep", status: tools.StatusOK, detail: "internal/cli/root.go:41: fs := flag.NewFlagSet(\"rune\", flag.ContinueOnError)"},
			transcriptRow{kind: rowToolResult, id: "c2", tool: "edit_file", status: tools.StatusOK, detail: diff},
			transcriptRow{kind: rowSystem, text: "Mode set to ask."},
			transcriptRow{kind: rowAskUser, id: "ask1", text: "ask_user: which of these very long alternative naming schemes should the new flag adopt", detail: "1. choose between --version and --print-version  (--version, --print-version, keep both and alias them)"},
			transcriptRow{kind: rowPermission, id: "p1", permission: &permissionEventLongDetailFixture},
			transcriptRow{kind: rowAssistant, text: "Done — the change is in.", final: true, turnTools: 2},
		)

		for index, line := range strings.Split(viewString(m.View()), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d: frame line %d is %d cells wide: %q", width, index, got, line)
			}
		}
	}
}

var permissionEventLongDetailFixture = agent.PermissionEvent{
	ToolCallID:     "p1",
	ToolName:       "bash",
	Action:         agent.PermissionActionPrompt,
	Permission:     "prompt",
	PermissionMode: agent.PermissionModeAsk,
	SideEffect:     "runs `go test ./... -timeout 600s` in /Users/dev/rune-project-workspace with network access",
	Reason:         "command writes outside the workspace and downloads modules from the network proxy",
	Risk:           sandbox.Risk{Level: sandbox.RiskMedium},
}
