package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type helpGroup struct {
	title    string
	bindings []key.Binding
}

func (a *App) helpGroups() []helpGroup {
	switch a.mode {
	case modeDetail:
		return []helpGroup{
			{title: "Navigation", bindings: []key.Binding{appKeys.scroll, appKeys.pageUp, appKeys.pageDown, appKeys.back}},
			{title: "View", bindings: []key.Binding{appKeys.allFields}},
			{title: "Job actions", bindings: []key.Binding{appKeys.cancel, appKeys.hold, appKeys.requeue, appKeys.edit}},
			{title: "App", bindings: []key.Binding{appKeys.help, appKeys.quit}},
		}
	case modePicker:
		view := []key.Binding{appKeys.toggle, appKeys.moveUp, appKeys.moveDown, appKeys.save, appKeys.back}
		if !a.pickerColumns {
			view = []key.Binding{appKeys.enter, appKeys.back}
		}
		return []helpGroup{
			{title: "Navigation", bindings: []key.Binding{appKeys.up, appKeys.down, appKeys.pageUp, appKeys.pageDown}},
			{title: "View", bindings: view},
			{title: "App", bindings: []key.Binding{appKeys.help, appKeys.quit}},
		}
	default:
		return []helpGroup{
			{title: "Navigation", bindings: []key.Binding{appKeys.up, appKeys.down, appKeys.pageUp, appKeys.pageDown, appKeys.enter}},
			{title: "View", bindings: []key.Binding{appKeys.tab, appKeys.queue, appKeys.recent, appKeys.filter, appKeys.sort, appKeys.columns}},
			{title: "Job actions", bindings: []key.Binding{appKeys.cancel, appKeys.hold, appKeys.requeue, appKeys.edit}},
			{title: "App", bindings: []key.Binding{appKeys.refresh, appKeys.scope, appKeys.help, appKeys.quit}},
		}
	}
}

func (a *App) renderHelpOverlay(width, height int) (string, int, int) {
	if height <= 0 {
		height = 24
	}
	panelWidth := min(max(1, width-4), 72)
	innerWidth := max(1, panelWidth-4)
	var body []string
	full := help.New()
	full.Width = innerWidth
	for _, group := range a.helpGroups() {
		body = append(body, titleStyle.Render(group.title))
		rendered := full.FullHelpView([][]key.Binding{group.bindings})
		for _, line := range strings.Split(rendered, "\n") {
			if line != "" {
				body = append(body, line)
			}
		}
	}

	visibleRows := max(1, height-6)
	maxScroll := max(0, len(body)-visibleRows)
	a.helpScroll = min(a.helpScroll, maxScroll)
	end := min(len(body), a.helpScroll+visibleRows)
	content := []string{titleStyle.Render("Key bindings")}
	content = append(content, body[a.helpScroll:end]...)
	content = append(content, mutedStyle.Render("↑/↓ or j/k scroll • ?/esc close"))
	for i := range content {
		content[i] = ansi.Truncate(content[i], innerWidth, "…")
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(innerWidth + 2).
		Render(strings.Join(content, "\n"))
	panelHeight := lipgloss.Height(panel)
	return panel, lipgloss.Width(panel), max(0, (height-panelHeight)/2)
}

func compositeOverlay(background, panel string, width, panelWidth, panelTop int) string {
	baseLines := strings.Split(background, "\n")
	panelLines := strings.Split(panel, "\n")
	left := max(0, (width-panelWidth)/2)
	right := left + panelWidth
	lines := append([]string(nil), baseLines...)
	for panelRow, panelLine := range panelLines {
		row := panelTop + panelRow
		if row >= len(lines) {
			lines = append(lines, make([]string, row-len(lines)+1)...)
		}
		baseLine := lines[row]
		baseWidth := lipgloss.Width(baseLine)
		prefix := ansi.Cut(baseLine, 0, left)
		if baseWidth < left {
			prefix += strings.Repeat(" ", left-baseWidth)
		}
		suffix := ""
		if baseWidth > right {
			suffix = strings.TrimRight(ansi.Cut(baseLine, right, width), " ")
		}
		lines[row] = prefix + panelLine + suffix
	}
	return strings.Join(lines, "\n")
}

// fitFooter keeps the actions at the front in priority order and reserves the
// final slot for help. If the terminal cannot fit the label, keep only '?'.
func fitFooter(width int, actions []string) string {
	const gap = "  "
	helpLabel := "? help"
	if width < lipgloss.Width(helpLabel) {
		return "?"
	}
	items := make([]string, 0, len(actions)+1)
	used := lipgloss.Width(helpLabel)
	for _, action := range actions {
		need := lipgloss.Width(action)
		if len(items) > 0 {
			need += lipgloss.Width(gap)
		}
		if used+need+lipgloss.Width(gap) > width {
			break
		}
		items = append(items, action)
		used += need
	}
	items = append(items, helpLabel)
	return strings.Join(items, gap)
}

func fitOutput(view string, width int) string {
	if width < 1 {
		return ""
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = ansi.Truncate(line, width, "…")
		}
	}
	return strings.Join(lines, "\n")
}
