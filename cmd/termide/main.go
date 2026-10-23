package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"termide/internal/client"
	"termide/internal/domain"
	"termide/internal/ui"
)

func main() {
	width := flag.Int("width", 80, "terminal width")
	height := flag.Int("height", 24, "terminal height")
	host := flag.String("ssh", "", "SSH host alias configured in ~/.ssh/config")
	project := flag.String("project", "", "absolute project directory on the VPS")
	flag.Parse()
	if (*host == "") != (*project == "") || (*project != "" && !strings.HasPrefix(*project, "/")) {
		fmt.Fprintln(os.Stderr, "--ssh and --project must be supplied together; project must be absolute")
		os.Exit(2)
	}
	if *width != 80 || *height != 24 {
		layout := ui.LayoutFor(*width, *height)
		fmt.Fprintf(os.Stdout, "TermIDE\nmain=%dx%d side=%dx%d\n", layout.Main.Width, layout.Main.Height, layout.Side.Width, layout.Side.Height)
		return
	}
	model := ui.NewModel()
	if *host != "" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		remote, err := client.DialSSH(ctx, *host)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer remote.Close()
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(*host+"\x00"+*project)))
		model = ui.NewRemoteModel(remote, domain.Workspace{ID: id, Host: *host, Root: *project, Name: *project})
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
