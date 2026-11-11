package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"slurmtui/internal/config"
	"slurmtui/internal/slurm"
	"slurmtui/internal/ui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	refresh := flag.String("refresh", "", "queue refresh interval (overrides config)")
	window := flag.String("window", "", "recent jobs window (overrides config)")
	all := flag.Bool("all", false, "show all users by default (overrides config)")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	settings, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "slurmtui:", err)
		os.Exit(2)
	}
	settings, err = config.Override(settings, *refresh, *window, *all)
	if err != nil {
		fmt.Fprintln(os.Stderr, "slurmtui:", err)
		os.Exit(2)
	}

	program := tea.NewProgram(ui.NewWithOptions(slurm.ExecRunner{}, ui.Options{
		RefreshInterval: settings.RefreshInterval,
		RecentWindow:    settings.RecentWindow,
		Scope:           settings.DefaultScope,
	}), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "slurmtui:", err)
		os.Exit(1)
	}
}
