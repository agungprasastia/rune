package tui

import "strings"

type SidebarMode uint8

const (
	SidebarAuto SidebarMode = iota
	SidebarShown
	SidebarHidden
)

type Layout struct {
	Width   int
	Height  int
	Main    tuiRect
	Sidebar tuiRect
	Footer  tuiRect
	Mode    SidebarMode
}

func (m model) layout() Layout {
	width := maxInt(m.width, 1)
	height := maxInt(m.height, 1)
	mode := m.sidebarDisplayMode()
	sidebar := 0
	if mode != SidebarHidden && width >= 120 && m.sidebarAvailable() {
		sidebar = sidebarWidth(width)
	}
	mainWidth := width
	if sidebar > 0 {
		mainWidth -= sidebar + 1
		if mainWidth < 60 {
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

func (l Layout) SidebarVisible() bool {
	return l.Sidebar.width > 0
}

func (l Layout) MainWidth() int {
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
