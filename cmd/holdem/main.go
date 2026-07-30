// Command holdem is a terminal Texas Hold'em game built as a learning tool.
//
// The TUI is not wired up yet — see docs/ for the design that this entry point
// will grow into.
package main

import "fmt"

// version is overwritten at release time via -ldflags.
var version = "dev"

func main() {
	fmt.Printf("holdem %s — under construction; see docs/DESIGN.md\n", version)
}
