package main

import (
	"flag"
	"log"

	"github.com/janderland/scraper/internal/gui"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/tags"
)

func main() {
	// Command line flags
	dbPath := flag.String("db", "./scraper.db", "Path to SQLite database")
	flag.Parse()

	// Initialize database
	db, err := storage.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize tagger
	tagger := tags.NewTagger(db)

	// Create and run GUI
	app := gui.NewGUI(db, tagger)
	app.Run()
}
