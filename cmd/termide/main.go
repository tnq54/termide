package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
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
	local := flag.Bool("local", false, "connect to the supervisor on this machine (no SSH)")
	project := flag.String("project", "", "absolute project directory on the VPS")
	flag.Parse()
	if (*local && *host != "") || ((*local || *host != "") != (*project != "")) || (*project != "" && !strings.HasPrefix(*project, "/")) {
		fmt.Fprintln(os.Stderr, "supply either --local or --ssh with --project; project must be absolute")
		os.Exit(2)
	}
	if *width != 80 || *height != 24 {
		layout := ui.LayoutFor(*width, *height)
		fmt.Fprintf(os.Stdout, "TermIDE\nmain=%dx%d side=%dx%d\n", layout.Main.Width, layout.Main.Height, layout.Side.Width, layout.Side.Height)
		return
	}
	model := ui.NewModel()
	if *host != "" || *local {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		dial := func(ctx context.Context) (io.ReadWriteCloser, error) {
			return client.DialSSHStream(ctx, *host)
		}
		identity := *host
		if *local {
			dial = client.DialLocalStream
			identity = "local"
		}
		stream, err := dial(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		remote := client.NewRemote(stream)
		defer remote.Close()
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(identity+"\x00"+*project)))
		model = ui.NewRemoteModel(remote, domain.Workspace{ID: id, Host: identity, Root: *project, Name: *project}).WithStreamDialer(dial)
	}
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
