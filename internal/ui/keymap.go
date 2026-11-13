package ui

import "github.com/charmbracelet/bubbles/key"

type appKeyMap struct {
	quit      key.Binding
	help      key.Binding
	filter    key.Binding
	sort      key.Binding
	columns   key.Binding
	cancel    key.Binding
	hold      key.Binding
	requeue   key.Binding
	edit      key.Binding
	toggle    key.Binding
	moveUp    key.Binding
	moveDown  key.Binding
	save      key.Binding
	tab       key.Binding
	queue     key.Binding
	recent    key.Binding
	up        key.Binding
	down      key.Binding
	pageUp    key.Binding
	pageDown  key.Binding
	scroll    key.Binding
	refresh   key.Binding
	scope     key.Binding
	enter     key.Binding
	back      key.Binding
	allFields key.Binding
}

var appKeys = appKeyMap{
	quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	sort:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
	columns:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "columns")),
	cancel:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancel")),
	hold:      key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "hold/release")),
	requeue:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "requeue")),
	edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit job")),
	toggle:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle column")),
	moveUp:    key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "move column up")),
	moveDown:  key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "move column down")),
	save:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "save columns")),
	tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next view")),
	queue:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "queue view")),
	recent:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "recent view")),
	up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "select previous")),
	down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "select next")),
	pageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	pageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	scroll:    key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓ or j/k", "scroll")),
	refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	scope:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "toggle scope")),
	enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	allFields: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all fields")),
}
