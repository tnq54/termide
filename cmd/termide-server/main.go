package main

import (
	"flag"
	"log"
	"os"

	"termide/internal/store"
)

func main() {
	database := flag.String("database", "termide.db", "SQLite database path")
	flag.Parse()
	s, err := store.Open(*database)
	if err != nil { log.Fatal(err) }
	defer s.Close()
	log.Printf("termide-server ready on %s", *database)
	if _, err := os.Stdout.WriteString("ready\n"); err != nil { log.Fatal(err) }
}
