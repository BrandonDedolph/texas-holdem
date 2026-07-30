// Command holdem is a terminal Texas Hold'em game built as a learning tool.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrandonDedolph/texas-holdem/internal/app"
)

// version is overwritten at release time via -ldflags.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("holdem %s\n", version)
		return
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "holdem: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// tea.WithAltScreen keeps the game off the scrollback so quitting leaves
	// the terminal as it was found.
	p := tea.NewProgram(app.New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
