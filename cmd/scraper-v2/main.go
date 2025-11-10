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
	configPath := flag.String("config", "./config/vpn.yaml", "Path to YAML configuration file")
	vpnStatus := flag.Bool("vpn-status", false, "Show VPN status and exit")

	flag.Parse()

	// Load configuration
	config, err := vpn.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database from config
	db, err := storage.NewDatabase(config.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize file storage from config
	fs, err := storage.NewFileStorage(config.Storage.MediaDir)
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}

	// Initialize VPN manager with new implementation
	vpnManager, err := vpn.NewManagerV2(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize VPN manager: %v", err)
	}
	defer vpnManager.Close()

	// Show VPN status if requested
	if *vpnStatus {
		showVPNStatus(vpnManager)
		return
	}

	// Initialize tagger
	tagger := tags.NewTagger(db)

	// Validate required parameters
	if *source == "" {
		log.Fatal("Source is required (use -source flag)")
	}

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
		scrapeInstagram(ctx, db, fs, vpnManager, tagger, *source, *maxPosts, *oldestFirst, config)

	case "facebook":
		scrapeFacebook(ctx, db, fs, vpnManager, tagger, *source, *maxPosts, *oldestFirst, startTime, endTime, config)

	default:
		log.Fatalf("Unknown platform: %s", *platform)
	}
}

func scrapeReddit(ctx context.Context, db *storage.Database, fs *storage.FileStorage,
	vpnManager *vpn.ManagerV2, tagger *tags.Tagger, source string, maxPosts int,
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
	vpnManager *vpn.ManagerV2, tagger *tags.Tagger, username string, maxPosts int,
	oldestFirst bool, config *vpn.Config) {

	log.Printf("Starting Instagram scraper for: %s", username)

	// Load session ID from config directory
	sessionID := loadInstagramConfig("./config")

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
	vpnManager *vpn.ManagerV2, tagger *tags.Tagger, username string, maxPosts int,
	oldestFirst bool, startDate, endDate time.Time, config *vpn.Config) {

	log.Printf("Starting Facebook scraper for: %s", username)

	// Load access token from config directory
	accessToken := loadFacebookConfig("./config")

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
	sessionFile := fmt.Sprintf("%s/instagram_session.txt", configPath)
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		log.Printf("Warning: Could not load Instagram session ID: %v", err)
		return ""
	}
	return string(data)
}

func loadFacebookConfig(configPath string) string {
	tokenFile := fmt.Sprintf("%s/facebook_token.txt", configPath)
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		log.Printf("Warning: Could not load Facebook access token: %v", err)
		return ""
	}
	return string(data)
}

func showVPNStatus(manager *vpn.ManagerV2) {
	fmt.Println("=== VPN Status ===\n")

	connected := manager.GetConnectedVPNs()
	if len(connected) == 0 {
		fmt.Println("No VPNs currently connected")
	} else {
		fmt.Printf("Connected VPNs: %d\n", len(connected))
		for _, name := range connected {
			fmt.Printf("  - %s\n", name)
		}
	}

	fmt.Println("\n=== VPN Statistics ===\n")

	stats := manager.GetStats()
	for name, stat := range stats {
		fmt.Printf("VPN: %s\n", name)
		fmt.Printf("  Connected: %v\n", stat.Connected)
		fmt.Printf("  Requests: %d\n", stat.RequestCount)
		fmt.Printf("  Bytes: %d\n", stat.BytesTransferred)
		fmt.Printf("  Failures: %d\n", stat.FailureCount)
		if !stat.LastUsed.IsZero() {
			fmt.Printf("  Last Used: %s\n", stat.LastUsed.Format("2006-01-02 15:04:05"))
		}
		if !stat.LastHealthCheck.IsZero() {
			fmt.Printf("  Last Health Check: %s (OK: %v)\n",
				stat.LastHealthCheck.Format("2006-01-02 15:04:05"),
				stat.HealthCheckOK)
		}
		fmt.Println()
	}
}
