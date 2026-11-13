package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

type appKeyMap struct {
	quit      key.Binding
	help      key.Binding
	filter    key.Binding
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
	tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab/1/2", "view")),
	queue:     key.NewBinding(key.WithKeys("1")),
	recent:    key.NewBinding(key.WithKeys("2")),
	up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/↓", "select")),
	down:      key.NewBinding(key.WithKeys("down", "j")),
	pageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup/pgdn", "page")),
	pageDown:  key.NewBinding(key.WithKeys("pgdown")),
	scroll:    key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓ or j/k", "scroll")),
	refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	scope:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "scope")),
	enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	allFields: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all fields")),
}

func (appKeyMap) listHelp() string {
	return strings.Join([]string{
		appKeys.tab.Help().Key + " " + appKeys.tab.Help().Desc,
		appKeys.up.Help().Key + " select",
		appKeys.enter.Help().Key + " " + appKeys.enter.Help().Desc,
		appKeys.refresh.Help().Key + " " + appKeys.refresh.Help().Desc,
		appKeys.scope.Help().Key + " " + appKeys.scope.Help().Desc,
		appKeys.filter.Help().Key + " " + appKeys.filter.Help().Desc,
		appKeys.help.Help().Key + " " + appKeys.help.Help().Desc,
		appKeys.quit.Help().Key + " " + appKeys.quit.Help().Desc,
	}, "  ")
}

func (appKeyMap) detailHelp() string {
	return strings.Join([]string{
		appKeys.scroll.Help().Key + " " + appKeys.scroll.Help().Desc,
		appKeys.pageUp.Help().Key + " " + appKeys.pageUp.Help().Desc,
		appKeys.allFields.Help().Key + " " + appKeys.allFields.Help().Desc,
		appKeys.back.Help().Key + " " + appKeys.back.Help().Desc,
		appKeys.quit.Help().Key + " " + appKeys.quit.Help().Desc,
	}, "  ")
}
