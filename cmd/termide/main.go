package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/ui"
)

func main() {
	width := flag.Int("width", 80, "terminal width")
	height := flag.Int("height", 24, "terminal height")
	flag.Parse()
	if *width != 80 || *height != 24 {
		layout := ui.LayoutFor(*width, *height)
		fmt.Fprintf(os.Stdout, "TermIDE\nmain=%dx%d side=%dx%d\n", layout.Main.Width, layout.Main.Height, layout.Side.Width, layout.Side.Height)
		return
	}
	if _, err := tea.NewProgram(ui.NewModel(), tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
