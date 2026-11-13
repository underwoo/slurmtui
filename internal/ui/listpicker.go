package ui

import (
	"fmt"
	"strings"
)

type columnSort struct {
	columnID   string
	active     bool
	descending bool
}

// listPicker is a small reusable selection list for sort and column pickers.
type listPicker struct {
	title     string
	items     []string
	cursor    int
	checked   []bool
	checkable bool
}

func newListPicker(title string, items []string, cursor int) listPicker {
	return listPicker{title: title, items: append([]string(nil), items...), cursor: max(0, min(cursor, len(items)-1))}
}

func (p *listPicker) Move(delta int) {
	if len(p.items) == 0 {
		p.cursor = 0
		return
	}
	p.cursor = max(0, min(len(p.items)-1, p.cursor+delta))
}

func (p *listPicker) Toggle() {
	if p.cursor >= 0 && p.cursor < len(p.checked) {
		p.checked[p.cursor] = !p.checked[p.cursor]
	}
}

func (p *listPicker) MoveItem(delta int) {
	to := p.cursor + delta
	if p.cursor < 0 || p.cursor >= len(p.items) || to < 0 || to >= len(p.items) {
		return
	}
	p.items[p.cursor], p.items[to] = p.items[to], p.items[p.cursor]
	if len(p.checked) == len(p.items) {
		p.checked[p.cursor], p.checked[to] = p.checked[to], p.checked[p.cursor]
	}
	p.cursor = to
}

func (p *listPicker) SetChecked(checked []bool) {
	p.checkable = true
	p.checked = append([]bool(nil), checked...)
}

func (p listPicker) Items() []string { return append([]string(nil), p.items...) }

func (p listPicker) Checked() []bool { return append([]bool(nil), p.checked...) }

func (p listPicker) Cursor() int { return p.cursor }

func (p listPicker) View(width, height int) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	var out strings.Builder
	fmt.Fprintln(&out, titleStyle.Render(p.title))
	visible := max(1, height-5)
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := min(len(p.items), start+visible)
	for i := start; i < end; i++ {
		marker := ""
		if p.checkable {
			marker = "[ ] "
			if i < len(p.checked) && p.checked[i] {
				marker = "[x] "
			}
		}
		prefix := "  "
		if i == p.cursor {
			prefix = "> "
		}
		line := prefix + marker + p.items[i]
		line = truncateLine(line, width)
		if i == p.cursor {
			line = selectedStyle.Render(line)
		}
		fmt.Fprintln(&out, line)
	}
	return strings.TrimSuffix(out.String(), "\n")
}
