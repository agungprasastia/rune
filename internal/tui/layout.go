package tui

import "strings"

type SidebarMode uint8

const (
	SidebarAuto SidebarMode = iota
	SidebarShown
	SidebarHidden
)

// ShellLayout is the canonical terminal geometry. Column allocation and
// vertical regions live here so rendering and input cannot drift apart.
type ShellLayout struct {
	Width   int
	Height  int
	Main    tuiRect
	Sidebar tuiRect
	Footer  tuiRect
	Mode    SidebarMode

	headerRect      tuiRect
	bodyRect        tuiRect
	footerRect      tuiRect
	composerRect    tuiRect
	statusRect      tuiRect
	headerLines     []string
	bodyHeight      int
	footerLines     []string
	fullFooterLines []string
	footerClip      int
}

func (l ShellLayout) OverlayRect(height int) tuiRect {
	if height <= 0 || l.bodyRect.height <= 0 {
		return tuiRect{}
	}
	visible := minInt(height, l.bodyRect.height)
	return tuiRect{x: l.bodyRect.x, y: l.bodyRect.y + (l.bodyRect.height-visible)/2, width: l.Main.width, height: visible}
}

// Layout remains an alias for package-local callers while ShellLayout is the
// single geometry type.
type Layout = ShellLayout

func (m model) layout() ShellLayout {
	width := chatWidth(m.width)
	height := maxInt(m.height, 1)
	mode := m.sidebarDisplayMode()
	sidebar := 0
	if mode != SidebarHidden && m.sidebarAvailable() {
		sidebar = sidebarWidth(width)
	}
	mainWidth := width
	if sidebar > 0 {
		mainWidth -= sidebar + 1
		if mainWidth < sidebarMinMainWidth {
			sidebar = 0
			mainWidth = width
		}
	}
	return Layout{
		Width:  width,
		Height: height,
		Main:   tuiRect{width: mainWidth, height: height},
		Sidebar: tuiRect{
			x:      mainWidth + 1,
			width:  sidebar,
			height: height,
		},
		Mode: mode,
	}
}

func (m model) sidebarDisplayMode() SidebarMode {
	if m.sidebarHidden {
		return SidebarHidden
	}
	return m.sidebarMode
}

func (l ShellLayout) SidebarVisible() bool {
	return l.Sidebar.width > 0
}

func (l ShellLayout) MainWidth() int {
	return maxInt(l.Main.width, 1)
}

func (m model) composeLayout(main string) string {
	layout := m.layout()
	if !layout.SidebarVisible() {
		return main
	}
	mainLines := viewLines(main)
	sidebarLines := m.renderContextSidebar(layout.Sidebar.width, layout.Height)
	for len(mainLines) < layout.Height {
		mainLines = append(mainLines, "")
	}
	if len(mainLines) > layout.Height {
		mainLines = mainLines[:layout.Height]
	}
	lines := make([]string, layout.Height)
	for i := range lines {
		left := padStyledLine(mainLines[i], layout.MainWidth())
		right := ""
		if i < len(sidebarLines) {
			right = sidebarLines[i]
		}
		lines[i] = left + runeTheme.line.Render("│") + padStyledLine(right, layout.Sidebar.width)
	}
	return strings.Join(lines, "\n")
}
