package main

import (
	"flag"
	"fmt"
	"os"

	"termide/internal/ui"
)

func main() {
	width := flag.Int("width", 80, "terminal width")
	height := flag.Int("height", 24, "terminal height")
	flag.Parse()
	layout := ui.LayoutFor(*width, *height)
	fmt.Fprintf(os.Stdout, "TermIDE\nmain=%dx%d side=%dx%d\n", layout.Main.Width, layout.Main.Height, layout.Side.Width, layout.Side.Height)
}
