package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/scraper/facebook"
	"github.com/janderland/scraper/internal/scraper/instagram"
	"github.com/janderland/scraper/internal/scraper/reddit"
	"github.com/janderland/scraper/internal/storage"
	"github.com/janderland/scraper/internal/tags"
	"github.com/janderland/scraper/internal/vpn"
)

func main() {
	// Command line flags
	platform := flag.String("platform", "reddit", "Platform to scrape (reddit, instagram, facebook)")
	source := flag.String("source", "", "Source to scrape (username, subreddit, etc.)")
	maxPosts := flag.Int("max", 100, "Maximum number of posts to scrape")
	minUpvotes := flag.Int("upvotes", 0, "Minimum upvotes (Reddit only)")
	oldestFirst := flag.Bool("oldest-first", false, "Scrape from oldest to newest (Instagram, Facebook)")
	startDate := flag.String("start-date", "", "Start date (YYYY-MM-DD)")
	endDate := flag.String("end-date", "", "End date (YYYY-MM-DD)")
	dbPath := flag.String("db", "./scraper.db", "Path to SQLite database")
	downloadDir := flag.String("dir", "./downloads", "Download directory")
	configPath := flag.String("config", "./config", "Configuration directory")

	flag.Parse()

	// Validate required parameters
	if *source == "" {
		log.Fatal("Source is required (use -source flag)")
	}

	// Initialize database
	db, err := storage.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize file storage
	fs, err := storage.NewFileStorage(*downloadDir)
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}

	// Initialize VPN manager
	vpnManager, err := vpn.NewManager(db)
	if err != nil {
		log.Fatalf("Failed to initialize VPN manager: %v", err)
	}

	// Initialize tagger
	tagger := tags.NewTagger(db)

	// Parse dates
	var startTime, endTime time.Time
	if *startDate != "" {
		startTime, err = time.Parse("2006-01-02", *startDate)
		if err != nil {
			log.Fatalf("Invalid start date: %v", err)
		}
	}
	if *endDate != "" {
		endTime, err = time.Parse("2006-01-02", *endDate)
		if err != nil {
			log.Fatalf("Invalid end date: %v", err)
		}
	}

	// Create scraper based on platform
	ctx := context.Background()

	switch *platform {
	case "reddit":
		scrapeReddit(ctx, db, fs, vpnManager, tagger, *source, *maxPosts, *minUpvotes, startTime, endTime)

	case "instagram":
		scrapeInstagram(ctx, db, fs, vpnManager, tagger, *source, *maxPosts, *oldestFirst, *configPath)

	case "facebook":
		scrapeFacebook(ctx, db, fs, vpnManager, tagger, *source, *maxPosts, *oldestFirst, startTime, endTime, *configPath)

	default:
		log.Fatalf("Unknown platform: %s", *platform)
	}
}

func scrapeReddit(ctx context.Context, db *storage.Database, fs *storage.FileStorage,
	vpnManager *vpn.Manager, tagger *tags.Tagger, source string, maxPosts int,
	minUpvotes int, startDate, endDate time.Time) {

	log.Printf("Starting Reddit scraper for: %s", source)

	scraper := reddit.NewScraper(db, fs, vpnManager)

	filter := &models.RedditFilter{
		Source:     source,
		MaxPosts:   maxPosts,
		MinUpvotes: minUpvotes,
		StartDate:  startDate,
		EndDate:    endDate,
	}

	if err := scraper.Scrape(ctx, filter); err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Printf("Scraping completed successfully")
}

func scrapeInstagram(ctx context.Context, db *storage.Database, fs *storage.FileStorage,
	vpnManager *vpn.Manager, tagger *tags.Tagger, username string, maxPosts int,
	oldestFirst bool, configPath string) {

	log.Printf("Starting Instagram scraper for: %s", username)

	// Load session ID from config
	sessionID := loadInstagramConfig(configPath)

	scraper := instagram.NewScraper(db, fs, vpnManager, sessionID)

	filter := &models.InstagramFilter{
		Username:    username,
		MaxPosts:    maxPosts,
		OldestFirst: oldestFirst,
	}

	if err := scraper.Scrape(ctx, filter); err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Printf("Scraping completed successfully")
}

func scrapeFacebook(ctx context.Context, db *storage.Database, fs *storage.FileStorage,
	vpnManager *vpn.Manager, tagger *tags.Tagger, username string, maxPosts int,
	oldestFirst bool, startDate, endDate time.Time, configPath string) {

	log.Printf("Starting Facebook scraper for: %s", username)

	// Load access token from config
	accessToken := loadFacebookConfig(configPath)

	scraper := facebook.NewScraper(db, fs, vpnManager, accessToken)

	// Get filter suggestions if requested
	if os.Getenv("SHOW_SUGGESTIONS") == "true" {
		suggestions, err := scraper.GetFilterSuggestions(ctx, username)
		if err != nil {
			log.Printf("Warning: Could not get filter suggestions: %v", err)
		} else {
			fmt.Printf("\nFilter Suggestions:\n")
			fmt.Printf("Total posts available: %d\n", suggestions.TotalPosts)
			fmt.Printf("Date range: %s to %s\n",
				suggestions.DateRange.Oldest.Format("2006-01-02"),
				suggestions.DateRange.Newest.Format("2006-01-02"))
			fmt.Printf("Suggested ranges: %v\n", suggestions.SuggestedRanges)
			fmt.Printf("Posts by year: %v\n", suggestions.PostsByYear)
			fmt.Println()
		}
	}

	filter := &models.FacebookFilter{
		Username:    username,
		MaxPosts:    maxPosts,
		StartDate:   startDate,
		EndDate:     endDate,
		OldestFirst: oldestFirst,
	}

	if err := scraper.Scrape(ctx, filter); err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Printf("Scraping completed successfully")
}

func loadInstagramConfig(configPath string) string {
	// Load Instagram session ID from config file
	// For now, return empty string (user needs to configure)
	sessionFile := fmt.Sprintf("%s/instagram_session.txt", configPath)
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		log.Printf("Warning: Could not load Instagram session ID: %v", err)
		return ""
	}
	return string(data)
}

func loadFacebookConfig(configPath string) string {
	// Load Facebook access token from config file
	tokenFile := fmt.Sprintf("%s/facebook_token.txt", configPath)
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		log.Printf("Warning: Could not load Facebook access token: %v", err)
		return ""
	}
	return string(data)
}
