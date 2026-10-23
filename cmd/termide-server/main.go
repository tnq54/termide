package main

import (
	"context"
	"flag"
	"log"
	"os"

	"termide/internal/server"
	"termide/internal/store"
)

func main() {
	database := flag.String("database", "termide.db", "SQLite database path")
	stdio := flag.Bool("stdio", false, "serve framed control requests on stdin/stdout")
	flag.Parse()
	s, err := store.Open(*database)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	if *stdio {
		if err := server.NewHandler(s).Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Printf("termide-server ready on %s", *database)
	if _, err := os.Stdout.WriteString("ready\n"); err != nil {
		log.Fatal(err)
	}
}
