package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"termide/internal/agent"
	"termide/internal/server"
	"termide/internal/store"
)

func main() {
	database := flag.String("database", "termide.db", "SQLite database path")
	stdio := flag.Bool("stdio", false, "serve framed control requests on stdin/stdout")
	daemon := flag.Bool("daemon", false, "run durable task supervisor")
	socket := flag.String("socket", "termide.sock", "durable supervisor Unix socket path")
	opencode := flag.String("opencode", "opencode", "OpenCode executable used for start_task")
	flag.Parse()
	s, err := store.Open(*database)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	if *daemon {
		lock, err := server.AcquireDatabaseLock(*database + ".lock")
		if err != nil {
			log.Fatal(err)
		}
		defer lock.Close()
		defer os.Remove(*socket)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		runner := agent.NewOpenCodeRunner(*opencode, nil)
		if err := server.RunDaemon(ctx, s, *socket, runner, fmt.Sprintf("%d", os.Getpid())); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *stdio {
		control, err := server.DialControl(context.Background(), *socket)
		if err != nil {
			log.Fatalf("connect durable supervisor: %v", err)
		}
		if err := server.RunStdioBridge(context.Background(), os.Stdin, control); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Printf("termide-server ready on %s", *database)
	if _, err := os.Stdout.WriteString("ready\n"); err != nil {
		log.Fatal(err)
	}
}
