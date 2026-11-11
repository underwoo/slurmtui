package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"slurmtui/internal/model"
	"slurmtui/internal/registry"
	"slurmtui/internal/slurm"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedStyle = lipgloss.NewStyle().Reverse(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

type Options struct {
	RefreshInterval time.Duration
	RecentWindow    time.Duration
	Scope           slurm.Scope
	Clock           func() time.Time
}

type App struct {
	runner  slurm.Runner
	options Options
	clock   func() time.Time

	width, height int
	showHelp      bool
	tab           registry.Tab
	scope         slurm.Scope
	queueJobs     []model.Job
	recentJobs    []model.Job
	queueCursor   int
	recentCursor  int
	queueLoading  bool
	recentLoading bool
	queueError    error
	recentError   error
	lastRefresh   time.Time
	detail        *model.JobDetail
	detailJob     model.Job
	detailLoading bool
	detailError   error
	detailScroll  int
	showAllFields bool
	detailRequest uint64
}

func New() tea.Model {
	return NewWithOptions(slurm.ExecRunner{}, Options{})
}

func NewWithOptions(runner slurm.Runner, options Options) *App {
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = 15 * time.Second
	}
	if options.RecentWindow <= 0 {
		options.RecentWindow = time.Hour
	}
	if options.Scope != slurm.ScopeAll {
		options.Scope = slurm.ScopeMe
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	return &App{runner: runner, options: options, clock: clock, tab: registry.QueueTab, scope: options.Scope}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.startQueueFetch(), a.startRecentFetch(), a.scheduleTick())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if a.detail != nil || a.detailLoading || a.detailError != nil {
			switch msg.String() {
			case "esc":
				a.detail, a.detailError = nil, nil
				a.detailLoading, a.showAllFields = false, false
				a.detailScroll = 0
				a.detailRequest++
			case "up", "k":
				a.detailScroll = max(0, a.detailScroll-1)
			case "down", "j":
				a.detailScroll++
			case "pgup":
				a.detailScroll = max(0, a.detailScroll-a.pageSize())
			case "pgdown":
				a.detailScroll += a.pageSize()
			case "a":
				a.showAllFields = !a.showAllFields
				a.detailScroll = 0
			case "q", "ctrl+c":
				return a, tea.Quit
			}
			return a, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
		case "tab":
			a.toggleTab()
		case "1":
			a.tab = registry.QueueTab
		case "2":
			a.tab = registry.RecentTab
		case "up", "k":
			a.moveCursor(-1)
		case "down", "j":
			a.moveCursor(1)
		case "pgup":
			a.moveCursor(-a.pageSize())
		case "pgdown":
			a.moveCursor(a.pageSize())
		case "r":
			return a, tea.Batch(a.startQueueFetch(), a.startRecentFetch())
		case "u":
			if a.scope == slurm.ScopeMe {
				a.scope = slurm.ScopeAll
			} else {
				a.scope = slurm.ScopeMe
			}
			return a, tea.Batch(a.startQueueFetch(), a.startRecentFetch())
		case "enter":
			_, jobs, cursor, _, _ := a.activeData()
			if len(jobs) > 0 && cursor >= 0 && cursor < len(jobs) {
				a.detailJob = jobs[cursor]
				a.detailLoading, a.detailError, a.detail = true, nil, nil
				a.detailScroll = 0
				a.detailRequest++
				return a, a.startDetailFetch(a.detailJob, a.tab == registry.RecentTab, a.detailRequest)
			}
		}
	case tickMsg:
		return a, tea.Batch(a.startQueueFetch(), a.startRecentFetch(), a.scheduleTick())
	case queueResult:
		a.queueLoading = false
		if msg.scope != a.scope {
			return a, a.startQueueFetch()
		}
		a.queueError = msg.err
		if msg.err == nil {
			a.queueCursor = restoreCursor(a.queueJobs, msg.result.Jobs, a.queueCursor)
			a.queueJobs = msg.result.Jobs
			a.lastRefresh = msg.at
		}
	case recentResult:
		a.recentLoading = false
		if msg.scope != a.scope {
			return a, a.startRecentFetch()
		}
		a.recentError = msg.err
		if msg.err == nil {
			a.recentCursor = restoreCursor(a.recentJobs, msg.result.Jobs, a.recentCursor)
			a.recentJobs = msg.result.Jobs
			a.lastRefresh = msg.at
		}
	case detailResult:
		if msg.request != a.detailRequest {
			break
		}
		a.detailLoading = false
		a.detailError = msg.err
		if msg.err == nil {
			detail := msg.detail
			a.detail = &detail
		}
	}
	return a, nil
}

func (a *App) View() string {
	width := a.width
	if width <= 0 {
		width = 120
	}
	var out strings.Builder
	if a.detail != nil || a.detailLoading || a.detailError != nil {
		a.renderDetail(&out, width)
		return out.String()
	}
	tabName, jobs, cursor, loading, sourceErr := a.activeData()
	fmt.Fprintf(&out, "%s  %s\n", titleStyle.Render("Slurm TUI"), mutedStyle.Render("scope: "+string(a.scope)))
	var queueTab, recentTab string
	if a.tab == registry.QueueTab {
		queueTab = titleStyle.Render("[Queue]")
		recentTab = mutedStyle.Render("Recent")
	} else {
		queueTab = mutedStyle.Render("Queue")
		recentTab = titleStyle.Render("[Recent]")
	}
	fmt.Fprintf(&out, "%s  %s\n", queueTab, recentTab)

	if a.showHelp {
		fmt.Fprintln(&out, "tab / 1 / 2 switch views   ↑/↓ or j/k select   enter details   r refresh   u toggle scope   ? help   q quit")
	} else {
		columns := registry.VisibleColumns(a.tab)
		widths := fitWidths(columns, width)
		tableHeight := a.height - 5
		if tableHeight <= 0 {
			tableHeight = 20
		}
		a.renderTable(&out, columns, widths, jobs, cursor, tableHeight)
	}

	status := fmt.Sprintf("%s: %d jobs", tabName, len(jobs))
	if loading {
		status += "  refreshing…"
	}
	if !a.lastRefresh.IsZero() {
		status += "  updated " + a.lastRefresh.Format("15:04:05")
	}
	if sourceErr != nil {
		status += "  " + sourceErr.Error()
	}
	otherName, _, _, otherLoading, otherErr := a.otherData()
	if otherLoading {
		status += "  " + otherName + " refreshing…"
	}
	if otherErr != nil {
		status += "  " + otherName + ": " + otherErr.Error()
	}
	if sourceErr != nil || otherErr != nil {
		fmt.Fprintln(&out, errorStyle.Render(status))
	} else {
		fmt.Fprintln(&out, mutedStyle.Render(status))
	}
	fmt.Fprint(&out, mutedStyle.Render("tab/1/2 view  ↑/↓ select  enter details  r refresh  u scope  ? help  q quit"))
	return out.String()
}

func (a *App) renderDetail(out *strings.Builder, width int) {
	fmt.Fprintf(out, "%s\n", titleStyle.Render("Job detail: "+a.detailJob.DisplayID()))
	if a.detailLoading {
		fmt.Fprintln(out, "Loading job details…")
	} else if a.detailError != nil {
		fmt.Fprintln(out, errorStyle.Render("Could not load job details: "+a.detailError.Error()))
	} else {
		lines := make([]string, 0, 64)
		for _, section := range registry.DetailSections() {
			if section.ID == "all" && !a.showAllFields {
				continue
			}
			if section.Applies != nil && !section.Applies(*a.detail) {
				continue
			}
			lines = append(lines, "", titleStyle.Render(section.Title))
			lines = append(lines, section.Render(*a.detail, a.clock())...)
		}
		available := max(1, a.height-2)
		if a.height <= 0 {
			available = 20
		}
		maxScroll := max(0, len(lines)-available)
		a.detailScroll = min(a.detailScroll, maxScroll)
		end := min(len(lines), a.detailScroll+available)
		for _, line := range lines[a.detailScroll:end] {
			fmt.Fprintln(out, truncateLine(line, width))
		}
	}
	fmt.Fprintln(out, mutedStyle.Render("↑/↓ or j/k scroll  pgup/pgdn page  a all fields  esc back  q quit"))
}

func truncateLine(line string, width int) string {
	if width < 1 {
		return ""
	}
	runes := []rune(line)
	if len(runes) <= width {
		return line
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func (a *App) renderTable(out *strings.Builder, columns []registry.Column, widths []int, jobs []model.Job, cursor, tableHeight int) {
	now := a.clock()
	for i, column := range columns {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(pad(column.Header, widths[i]))
	}
	out.WriteByte('\n')
	for i, width := range widths {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(strings.Repeat("─", width))
	}
	out.WriteByte('\n')
	rowCapacity := tableHeight - 2
	if rowCapacity < 1 {
		rowCapacity = 1
	}
	if len(jobs) == 0 {
		out.WriteString("(no jobs)\n")
		return
	}
	start := 0
	if cursor >= rowCapacity {
		start = cursor - rowCapacity + 1
	}
	end := min(len(jobs), start+rowCapacity)
	for row := start; row < end; row++ {
		var line strings.Builder
		for columnIndex, column := range columns {
			if columnIndex > 0 {
				line.WriteByte(' ')
			}
			line.WriteString(pad(column.Value(jobs[row], now), widths[columnIndex]))
		}
		text := line.String()
		if row == cursor {
			text = selectedStyle.Render(text)
		}
		out.WriteString(text)
		out.WriteByte('\n')
	}
}

func pad(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > width {
		if width == 1 {
			return "…"
		}
		return string(runes[:width-1]) + "…"
	}
	return value + strings.Repeat(" ", width-len(runes))
}

func fitWidths(columns []registry.Column, totalWidth int) []int {
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = column.Width
	}
	available := max(1, totalWidth-(len(columns)-1))
	for sum(widths) > available {
		largest := -1
		for i := range widths {
			if widths[i] > columns[i].MinWidth && (largest < 0 || widths[i] > widths[largest]) {
				largest = i
			}
		}
		if largest < 0 {
			for i := range widths {
				if widths[i] > 1 && (largest < 0 || widths[i] > widths[largest]) {
					largest = i
				}
			}
		}
		if largest < 0 {
			break
		}
		widths[largest]--
	}
	return widths
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func (a *App) activeData() (string, []model.Job, int, bool, error) {
	if a.tab == registry.RecentTab {
		return "Recent", a.recentJobs, a.recentCursor, a.recentLoading, a.recentError
	}
	return "Queue", a.queueJobs, a.queueCursor, a.queueLoading, a.queueError
}

func (a *App) otherData() (string, []model.Job, int, bool, error) {
	if a.tab == registry.RecentTab {
		return "Queue", a.queueJobs, a.queueCursor, a.queueLoading, a.queueError
	}
	return "Recent", a.recentJobs, a.recentCursor, a.recentLoading, a.recentError
}

func (a *App) toggleTab() {
	if a.tab == registry.QueueTab {
		a.tab = registry.RecentTab
	} else {
		a.tab = registry.QueueTab
	}
}

func (a *App) moveCursor(delta int) {
	jobs := a.queueJobs
	cursor := &a.queueCursor
	if a.tab == registry.RecentTab {
		jobs, cursor = a.recentJobs, &a.recentCursor
	}
	if len(jobs) == 0 {
		*cursor = 0
		return
	}
	*cursor = max(0, min(len(jobs)-1, *cursor+delta))
}

func (a *App) pageSize() int {
	if a.height <= 5 {
		return 1
	}
	return a.height - 5
}

func restoreCursor(oldJobs, newJobs []model.Job, oldCursor int) int {
	if oldCursor < 0 {
		oldCursor = 0
	}
	if oldCursor >= len(oldJobs) {
		oldCursor = max(0, len(oldJobs)-1)
	}
	selectedID := ""
	if len(oldJobs) > 0 {
		selectedID = oldJobs[oldCursor].DisplayID()
	}
	if selectedID != "" {
		for i := range newJobs {
			if newJobs[i].DisplayID() == selectedID {
				return i
			}
		}
	}
	if len(newJobs) == 0 {
		return 0
	}
	return min(oldCursor, len(newJobs)-1)
}

func (a *App) startQueueFetch() tea.Cmd {
	if a.queueLoading || a.runner == nil {
		return nil
	}
	a.queueLoading = true
	runner, scope, clock := a.runner, a.scope, a.clock
	return func() tea.Msg {
		result, err := slurm.ListJobs(context.Background(), runner, scope)
		return queueResult{result: result, err: err, at: clock(), scope: scope}
	}
}

func (a *App) startRecentFetch() tea.Cmd {
	if a.recentLoading || a.runner == nil {
		return nil
	}
	a.recentLoading = true
	runner, scope, window, clock := a.runner, a.scope, a.options.RecentWindow, a.clock
	return func() tea.Msg {
		result, err := slurm.ListRecent(context.Background(), runner, scope, window, clock)
		return recentResult{result: result, err: err, at: clock(), scope: scope}
	}
}

func (a *App) startDetailFetch(job model.Job, fromRecent bool, request uint64) tea.Cmd {
	if a.runner == nil {
		a.detailLoading = false
		a.detailError = fmt.Errorf("slurm runner is unavailable")
		return nil
	}
	runner := a.runner
	return func() tea.Msg {
		detail, err := slurm.GetJobDetail(context.Background(), runner, job, fromRecent)
		return detailResult{detail: detail, err: err, request: request}
	}
}

func (a *App) scheduleTick() tea.Cmd {
	return tea.Tick(a.options.RefreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

type tickMsg struct{}
type queueResult struct {
	result slurm.ListResult
	err    error
	at     time.Time
	scope  slurm.Scope
}
type recentResult struct {
	result slurm.ListResult
	err    error
	at     time.Time
	scope  slurm.Scope
}
type detailResult struct {
	detail  model.JobDetail
	err     error
	request uint64
}
