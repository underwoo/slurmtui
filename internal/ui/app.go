package ui

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"slurmtui/internal/config"
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
	Location        *time.Location
	ConfigPath      string
	QueueColumns    []string
	RecentColumns   []string
}

type App struct {
	runner  slurm.Runner
	options Options
	clock   func() time.Time

	width, height  int
	mode           mode
	showHelp       bool
	tab            registry.Tab
	scope          slurm.Scope
	queueJobs      []model.Job
	recentJobs     []model.Job
	filteredQueue  []model.Job
	filteredRecent []model.Job
	queueSort      columnSort
	recentSort     columnSort
	picker         listPicker
	pickerColumns  bool
	columnWarning  string
	queueCursor    int
	recentCursor   int
	queueLoading   bool
	recentLoading  bool
	queueError     error
	recentError    error
	lastRefresh    time.Time
	detail         *model.JobDetail
	detailJob      model.Job
	detailError    error
	detailScroll   int
	showAllFields  bool
	detailRequest  uint64
	filterInput    textinput.Model
	filterPattern  string
	filterRegex    *regexp.Regexp
	filterError    error
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
	if options.Location == nil {
		options.Location = time.Local
	}
	input := textinput.New()
	input.Prompt = "/"
	input.Width = 80
	input.CharLimit = 256
	app := &App{runner: runner, options: options, clock: clock, tab: registry.QueueTab, scope: options.Scope, filterInput: input}
	app.refreshColumnWarnings()
	return app
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.startQueueFetch(), a.startRecentFetch(), a.scheduleTick())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.filterInput.Width = max(1, msg.Width-35)
	case tea.KeyMsg:
		return a, a.handleKey(msg)
	case tickMsg:
		return a, tea.Batch(a.startQueueFetch(), a.startRecentFetch(), a.scheduleTick())
	case queueResult:
		a.queueLoading = false
		if msg.scope != a.scope {
			return a, a.startQueueFetch()
		}
		a.queueError = msg.err
		if msg.err == nil {
			oldJobs := a.visibleQueue()
			a.queueJobs = msg.result.Jobs
			sortJobs(a.queueJobs, a.columnsFor(registry.QueueTab), a.queueSort)
			a.recomputeQueueFilter()
			a.queueCursor = restoreCursor(oldJobs, a.visibleQueue(), a.queueCursor)
			a.lastRefresh = msg.at
		}
	case recentResult:
		a.recentLoading = false
		if msg.scope != a.scope {
			return a, a.startRecentFetch()
		}
		a.recentError = msg.err
		if msg.err == nil {
			oldJobs := a.visibleRecent()
			a.recentJobs = msg.result.Jobs
			sortJobs(a.recentJobs, a.columnsFor(registry.RecentTab), a.recentSort)
			a.recomputeRecentFilter()
			a.recentCursor = restoreCursor(oldJobs, a.visibleRecent(), a.recentCursor)
			a.lastRefresh = msg.at
		}
	case detailResult:
		if msg.request != a.detailRequest {
			break
		}
		a.detailError = msg.err
		if msg.err == nil {
			detail := msg.detail
			a.detail = &detail
		}
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch a.mode {
	case modeDetail:
		return a.handleDetailKey(msg)
	case modeFilter:
		return a.handleFilterKey(msg)
	case modePicker:
		return a.handlePickerKey(msg)
	case modeConfirm:
		return a.handleConfirmKey(msg)
	case modeForm:
		return a.handleFormKey(msg)
	default:
		return a.handleListKey(msg)
	}
}

func (a *App) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, appKeys.quit):
		return tea.Quit
	case key.Matches(msg, appKeys.help):
		a.showHelp = !a.showHelp
	case key.Matches(msg, appKeys.filter):
		return a.beginFilter()
	case key.Matches(msg, appKeys.sort):
		a.beginSort()
	case key.Matches(msg, appKeys.columns):
		a.beginColumnPicker()
	case key.Matches(msg, appKeys.back):
		if a.hasFilterDraft() {
			a.clearFilter()
		}
	case key.Matches(msg, appKeys.tab):
		a.toggleTab()
	case key.Matches(msg, appKeys.queue):
		a.tab = registry.QueueTab
	case key.Matches(msg, appKeys.recent):
		a.tab = registry.RecentTab
	case key.Matches(msg, appKeys.up):
		a.moveCursor(-1)
	case key.Matches(msg, appKeys.down):
		a.moveCursor(1)
	case key.Matches(msg, appKeys.pageUp):
		a.moveCursor(-a.pageSize())
	case key.Matches(msg, appKeys.pageDown):
		a.moveCursor(a.pageSize())
	case key.Matches(msg, appKeys.refresh):
		return tea.Batch(a.startQueueFetch(), a.startRecentFetch())
	case key.Matches(msg, appKeys.scope):
		if a.scope == slurm.ScopeMe {
			a.scope = slurm.ScopeAll
		} else {
			a.scope = slurm.ScopeMe
		}
		return tea.Batch(a.startQueueFetch(), a.startRecentFetch())
	case key.Matches(msg, appKeys.enter):
		_, jobs, cursor, _, _ := a.activeData()
		if len(jobs) > 0 && cursor >= 0 && cursor < len(jobs) {
			a.detailJob = jobs[cursor]
			a.mode = modeDetail
			a.detailError, a.detail = nil, nil
			a.detailScroll = 0
			a.detailRequest++
			return a.startDetailFetch(a.detailJob, a.tab == registry.RecentTab, a.detailRequest)
		}
	}
	return nil
}

func (a *App) handleDetailKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, appKeys.back):
		a.mode = modeList
		a.detail, a.detailError = nil, nil
		a.showAllFields = false
		a.detailScroll = 0
		a.detailRequest++
	case key.Matches(msg, appKeys.up):
		a.detailScroll = max(0, a.detailScroll-1)
	case key.Matches(msg, appKeys.down):
		a.detailScroll++
	case key.Matches(msg, appKeys.pageUp):
		a.detailScroll = max(0, a.detailScroll-a.pageSize())
	case key.Matches(msg, appKeys.pageDown):
		a.detailScroll += a.pageSize()
	case key.Matches(msg, appKeys.allFields):
		a.showAllFields = !a.showAllFields
		a.detailScroll = 0
	case key.Matches(msg, appKeys.quit):
		return tea.Quit
	}
	return nil
}

func (a *App) handleFilterKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, appKeys.back):
		a.mode = modeList
		a.clearFilter()
		return nil
	case key.Matches(msg, appKeys.enter):
		a.mode = modeList
		a.filterInput.Blur()
		return nil
	default:
		before := a.filterInput.Value()
		var cmd tea.Cmd
		a.filterInput, cmd = a.filterInput.Update(msg)
		if a.filterInput.Value() != before {
			a.applyFilter(a.filterInput.Value())
		}
		return cmd
	}
}

func (a *App) beginFilter() tea.Cmd {
	if a.filterError == nil {
		a.filterInput.SetValue(a.filterPattern)
	}
	width := a.width
	if width <= 0 {
		width = 120
	}
	a.filterInput.Width = max(1, width-35)
	a.mode = modeFilter
	return a.filterInput.Focus()
}

func (a *App) beginSort() {
	a.pickerColumns = false
	columns := a.columnsFor(a.tab)
	items := make([]string, len(columns))
	cursor := 0
	state := a.activeSort()
	for i, column := range columns {
		items[i] = column.Header
		if state.active && column.ID == state.columnID {
			cursor = i
		}
	}
	a.picker = newListPicker("Sort by column", items, cursor)
	a.mode = modePicker
}

func (a *App) beginColumnPicker() {
	all := registry.VisibleColumns(a.tab)
	visible := a.columnsFor(a.tab)
	visibleIDs := make(map[string]bool, len(visible))
	for _, column := range visible {
		visibleIDs[column.ID] = true
	}
	items, checked := make([]string, len(all)), make([]bool, len(all))
	for i, column := range all {
		items[i], checked[i] = column.ID+"  "+column.Header, visibleIDs[column.ID]
	}
	a.picker = newListPicker("Choose columns (J/K reorder)", items, 0)
	a.picker.SetChecked(checked)
	a.pickerColumns = true
	a.mode = modePicker
}

func (a *App) columnsFor(tab registry.Tab) []registry.Column {
	ids := a.options.QueueColumns
	if tab == registry.RecentTab {
		ids = a.options.RecentColumns
	}
	columns, _ := registry.ColumnsForIDs(tab, ids)
	return columns
}

func (a *App) refreshColumnWarnings() {
	a.columnWarning = ""
	var unknown []string
	for _, tab := range []registry.Tab{registry.QueueTab, registry.RecentTab} {
		ids := a.options.QueueColumns
		if tab == registry.RecentTab {
			ids = a.options.RecentColumns
		}
		_, skipped := registry.ColumnsForIDs(tab, ids)
		unknown = append(unknown, skipped...)
	}
	if len(unknown) != 0 {
		a.columnWarning = "unknown column IDs skipped: " + strings.Join(uniqueStrings(unknown), ", ")
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (a *App) savePickedColumns() {
	oldQueue, oldRecent := append([]model.Job(nil), a.visibleQueue()...), append([]model.Job(nil), a.visibleRecent()...)
	items, checked := a.picker.Items(), a.picker.Checked()
	ids := make([]string, 0, len(items))
	for i, item := range items {
		if i < len(checked) && checked[i] {
			id, _, _ := strings.Cut(item, "  ")
			ids = append(ids, id)
		}
	}
	queue, recent := a.options.QueueColumns, a.options.RecentColumns
	if a.tab == registry.QueueTab {
		queue = ids
	} else {
		recent = ids
	}
	if queue == nil {
		queue = columnIDs(registry.QueueTab)
	}
	if recent == nil {
		recent = columnIDs(registry.RecentTab)
	}
	if err := config.SaveColumns(a.options.ConfigPath, queue, recent); err != nil {
		a.columnWarning = "could not save columns: " + err.Error()
		a.mode = modeList
		return
	}
	if a.tab == registry.QueueTab {
		a.options.QueueColumns = ids
	} else {
		a.options.RecentColumns = ids
	}
	a.refreshColumnWarnings()
	sortJobs(a.queueJobs, a.columnsFor(registry.QueueTab), a.queueSort)
	sortJobs(a.recentJobs, a.columnsFor(registry.RecentTab), a.recentSort)
	a.recomputeQueueFilter()
	a.recomputeRecentFilter()
	a.queueCursor = restoreCursor(oldQueue, a.visibleQueue(), a.queueCursor)
	a.recentCursor = restoreCursor(oldRecent, a.visibleRecent(), a.recentCursor)
	a.mode = modeList
}

func columnIDs(tab registry.Tab) []string {
	columns := registry.VisibleColumns(tab)
	ids := make([]string, len(columns))
	for i, column := range columns {
		ids[i] = column.ID
	}
	return ids
}

func (a *App) activeSort() *columnSort {
	if a.tab == registry.RecentTab {
		return &a.recentSort
	}
	return &a.queueSort
}

func (a *App) applyPickedSort() {
	columns := a.columnsFor(a.tab)
	index := a.picker.Cursor()
	if index < 0 || index >= len(columns) {
		a.mode = modeList
		return
	}
	oldQueue := append([]model.Job(nil), a.visibleQueue()...)
	oldRecent := append([]model.Job(nil), a.visibleRecent()...)
	state := a.activeSort()
	column := columns[index]
	if state.active && state.columnID == column.ID {
		state.descending = !state.descending
	} else {
		state.active = true
		state.columnID = column.ID
		state.descending = false
	}
	if a.tab == registry.RecentTab {
		sortJobs(a.recentJobs, columns, *state)
		a.recomputeRecentFilter()
		a.recentCursor = restoreCursor(oldRecent, a.visibleRecent(), a.recentCursor)
	} else {
		sortJobs(a.queueJobs, columns, *state)
		a.recomputeQueueFilter()
		a.queueCursor = restoreCursor(oldQueue, a.visibleQueue(), a.queueCursor)
	}
	a.mode = modeList
}

func sortJobs(jobs []model.Job, columns []registry.Column, state columnSort) {
	if !state.active {
		return
	}
	var selected registry.Column
	for _, column := range columns {
		if column.ID == state.columnID {
			selected = column
			break
		}
	}
	if selected.Less == nil {
		for _, column := range registry.DefaultColumns() {
			if column.ID == state.columnID {
				selected = column
				break
			}
		}
	}
	if selected.Less == nil {
		return
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		a, b := jobs[i], jobs[j]
		if selected.Less(a, b) {
			return !state.descending
		}
		if selected.Less(b, a) {
			return state.descending
		}
		return registry.JobIDLess(a, b)
	})
}

func (a *App) renderPicker(out *strings.Builder, width int) {
	fmt.Fprintf(out, "%s  %s\n", titleStyle.Render("Slurm TUI"), mutedStyle.Render("scope: "+string(a.scope)))
	queueTab, recentTab := mutedStyle.Render("Queue"), mutedStyle.Render("Recent")
	if a.tab == registry.QueueTab {
		queueTab = titleStyle.Render("[Queue]")
	} else {
		recentTab = titleStyle.Render("[Recent]")
	}
	fmt.Fprintf(out, "%s  %s\n", queueTab, recentTab)
	fmt.Fprintln(out, a.picker.View(width, a.height))
	if a.pickerColumns {
		fmt.Fprint(out, mutedStyle.Render("space toggle  J/K reorder  w save  esc cancel"))
	} else {
		fmt.Fprint(out, mutedStyle.Render("↑/↓ or j/k select  pgup/pgdn page  enter sort  esc cancel"))
	}
}

func (a *App) applyFilter(pattern string) {
	compiled, err := compileFilter(pattern)
	if err != nil {
		a.filterError = err
		return
	}
	oldQueue, oldRecent := a.visibleQueue(), a.visibleRecent()
	a.filterPattern = pattern
	a.filterRegex = compiled
	a.filterError = nil
	a.recomputeQueueFilter()
	a.recomputeRecentFilter()
	a.queueCursor = restoreCursor(oldQueue, a.visibleQueue(), a.queueCursor)
	a.recentCursor = restoreCursor(oldRecent, a.visibleRecent(), a.recentCursor)
}

func (a *App) clearFilter() {
	oldQueue, oldRecent := a.visibleQueue(), a.visibleRecent()
	a.filterPattern = ""
	a.filterRegex = nil
	a.filterError = nil
	a.filterInput.SetValue("")
	a.filterInput.Blur()
	a.filteredQueue, a.filteredRecent = nil, nil
	a.queueCursor = restoreCursor(oldQueue, a.queueJobs, a.queueCursor)
	a.recentCursor = restoreCursor(oldRecent, a.recentJobs, a.recentCursor)
}

func (a *App) hasFilterDraft() bool {
	return a.filterRegex != nil || a.filterError != nil || a.filterInput.Value() != ""
}

func (a *App) filterStatusVisible() bool {
	return a.mode == modeFilter || a.filterRegex != nil || a.filterError != nil
}

func (a *App) allActiveJobs() []model.Job {
	if a.tab == registry.RecentTab {
		return a.recentJobs
	}
	return a.queueJobs
}

func (a *App) visibleQueue() []model.Job {
	if a.filterRegex == nil {
		return a.queueJobs
	}
	return a.filteredQueue
}

func (a *App) visibleRecent() []model.Job {
	if a.filterRegex == nil {
		return a.recentJobs
	}
	return a.filteredRecent
}

func (a *App) recomputeQueueFilter() {
	if a.filterRegex == nil {
		a.filteredQueue = nil
		return
	}
	a.filteredQueue = filterJobs(a.queueJobs, a.columnsFor(registry.QueueTab), a.filterRegex, a.clock(), a.options.Location)
}

func (a *App) recomputeRecentFilter() {
	if a.filterRegex == nil {
		a.filteredRecent = nil
		return
	}
	a.filteredRecent = filterJobs(a.recentJobs, a.columnsFor(registry.RecentTab), a.filterRegex, a.clock(), a.options.Location)
}

func (a *App) handlePickerKey(msg tea.KeyMsg) tea.Cmd {
	if a.pickerColumns {
		switch {
		case key.Matches(msg, appKeys.back):
			a.mode = modeList
		case key.Matches(msg, appKeys.toggle):
			a.picker.Toggle()
		case key.Matches(msg, appKeys.moveUp):
			a.picker.MoveItem(-1)
		case key.Matches(msg, appKeys.moveDown):
			a.picker.MoveItem(1)
		case key.Matches(msg, appKeys.save):
			a.savePickedColumns()
		}
		return nil
	}
	switch {
	case key.Matches(msg, appKeys.back):
		a.mode = modeList
	case key.Matches(msg, appKeys.up):
		a.picker.Move(-1)
	case key.Matches(msg, appKeys.down):
		a.picker.Move(1)
	case key.Matches(msg, appKeys.pageUp):
		a.picker.Move(-a.pageSize())
	case key.Matches(msg, appKeys.pageDown):
		a.picker.Move(a.pageSize())
	case key.Matches(msg, appKeys.enter):
		a.applyPickedSort()
	}
	return nil
}

func (a *App) handleConfirmKey(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, appKeys.back) {
		a.mode = modeList
	}
	return nil
}

func (a *App) handleFormKey(msg tea.KeyMsg) tea.Cmd {
	if key.Matches(msg, appKeys.back) {
		a.mode = modeList
	}
	return nil
}

func (a *App) View() string {
	width := a.width
	if width <= 0 {
		width = 120
	}
	var out strings.Builder
	if a.mode == modeDetail {
		a.renderDetail(&out, width)
		return out.String()
	}
	if a.mode == modePicker {
		a.renderPicker(&out, width)
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
		fmt.Fprintln(&out, "tab / 1 / 2 switch views   ↑/↓ or j/k select   enter details   / filter   s sort   c columns   r refresh   u toggle scope   ? help   q quit")
	} else {
		columns := a.displayedColumns()
		widths := fitWidths(columns, width)
		tableHeight := a.height - 5
		if tableHeight <= 0 {
			tableHeight = 20
		}
		a.renderTable(&out, columns, widths, jobs, cursor, tableHeight)
	}

	status := fmt.Sprintf("%s: %d jobs", tabName, len(jobs))
	if a.filterStatusVisible() {
		allJobs := a.allActiveJobs()
		pattern := a.filterPattern
		if a.mode == modeFilter || a.filterError != nil {
			pattern = a.filterInput.Value()
		}
		if a.mode == modeFilter {
			status = a.filterInput.View()
		} else {
			status = fmt.Sprintf("%s  /%s", tabName, pattern)
		}
		status += fmt.Sprintf("  %d of %d", len(jobs), len(allJobs))
	}
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
	if a.filterError != nil {
		status += "  " + a.filterError.Error()
	}
	if a.columnWarning != "" {
		status += "  " + a.columnWarning
	}
	if sourceErr != nil || otherErr != nil || a.filterError != nil || a.columnWarning != "" {
		fmt.Fprintln(&out, errorStyle.Render(status))
	} else {
		fmt.Fprintln(&out, mutedStyle.Render(status))
	}
	fmt.Fprint(&out, mutedStyle.Render(appKeys.listHelp()))
	return out.String()
}

func (a *App) renderDetail(out *strings.Builder, width int) {
	fmt.Fprintf(out, "%s\n", titleStyle.Render("Job detail: "+a.detailJob.DisplayID()))
	if a.detail == nil && a.detailError != nil {
		fmt.Fprintln(out, errorStyle.Render("Could not load job details: "+a.detailError.Error()))
	} else if a.detail == nil {
		fmt.Fprintln(out, "Loading job details…")
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
			for _, line := range section.Render(*a.detail, a.clock(), a.options.Location) {
				if section.ID == "summary" && strings.HasPrefix(line, "Comment: ") {
					lines = append(lines, wrapCommentLine(line, width)...)
				} else {
					lines = append(lines, strings.Split(line, "\n")...)
				}
			}
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
	fmt.Fprintln(out, mutedStyle.Render(appKeys.detailHelp()))
}

func wrapCommentLine(line string, width int) []string {
	const prefix = "Comment: "
	comment := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	valueWidth := max(1, width-len([]rune(prefix)))
	continuation := strings.Repeat(" ", len([]rune(prefix)))
	wrapped := make([]string, 0, strings.Count(comment, "\n")+1)
	for _, paragraph := range strings.Split(comment, "\n") {
		if paragraph == "" {
			wrapped = append(wrapped, continuation)
			continue
		}
		words := strings.Fields(paragraph)
		current := ""
		for _, word := range words {
			for len([]rune(word)) > valueWidth {
				if current != "" {
					wrapped = append(wrapped, continuation+current)
					current = ""
				}
				runes := []rune(word)
				wrapped = append(wrapped, continuation+string(runes[:valueWidth]))
				word = string(runes[valueWidth:])
			}
			if current == "" {
				current = word
			} else if len([]rune(current))+1+len([]rune(word)) <= valueWidth {
				current += " " + word
			} else {
				wrapped = append(wrapped, continuation+current)
				current = word
			}
		}
		wrapped = append(wrapped, continuation+current)
	}
	if len(wrapped) > 0 {
		wrapped[0] = prefix + strings.TrimPrefix(wrapped[0], continuation)
	}
	return wrapped
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
			line.WriteString(pad(column.Value(jobs[row], now, a.options.Location), widths[columnIndex]))
		}
		text := line.String()
		if row == cursor {
			text = selectedStyle.Render(text)
		}
		out.WriteString(text)
		out.WriteByte('\n')
	}
}

func (a *App) displayedColumns() []registry.Column {
	columns := a.columnsFor(a.tab)
	state := a.activeSort()
	if state.active {
		for i := range columns {
			if columns[i].ID == state.columnID {
				arrow := " ▲"
				if state.descending {
					arrow = " ▼"
				}
				columns[i].Header += arrow
				break
			}
		}
	}
	return columns
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
		return "Recent", a.visibleRecent(), a.recentCursor, a.recentLoading, a.recentError
	}
	return "Queue", a.visibleQueue(), a.queueCursor, a.queueLoading, a.queueError
}

func (a *App) otherData() (string, []model.Job, int, bool, error) {
	if a.tab == registry.RecentTab {
		return "Queue", a.visibleQueue(), a.queueCursor, a.queueLoading, a.queueError
	}
	return "Recent", a.visibleRecent(), a.recentCursor, a.recentLoading, a.recentError
}

func (a *App) toggleTab() {
	if a.tab == registry.QueueTab {
		a.tab = registry.RecentTab
	} else {
		a.tab = registry.QueueTab
	}
}

func (a *App) moveCursor(delta int) {
	jobs := a.visibleQueue()
	cursor := &a.queueCursor
	if a.tab == registry.RecentTab {
		jobs, cursor = a.visibleRecent(), &a.recentCursor
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
