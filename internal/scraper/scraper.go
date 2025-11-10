package scraper

import (
	"context"
	"fmt"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
)

// Scraper defines the interface that all platform scrapers must implement
type Scraper interface {
	// Scrape performs the scraping operation
	Scrape(ctx context.Context, filter interface{}) error

	// GetProgress returns the current progress percentage (0-100)
	GetProgress() int

	// GetPlatform returns the platform this scraper handles
	GetPlatform() models.Platform

	// Stop gracefully stops the scraper
	Stop() error
}

// BaseScraper provides common functionality for all scrapers
type BaseScraper struct {
	db          *storage.Database
	fileStorage *storage.FileStorage
	vpnManager  VPNManager
	progress    int
	stopped     bool
}

// NewBaseScraper creates a new base scraper
func NewBaseScraper(db *storage.Database, fs *storage.FileStorage, vpn VPNManager) *BaseScraper {
	return &BaseScraper{
		db:          db,
		fileStorage: fs,
		vpnManager:  vpn,
		progress:    0,
		stopped:     false,
	}
}

// GetProgress returns the current progress
func (b *BaseScraper) GetProgress() int {
	return b.progress
}

// SetProgress sets the current progress
func (b *BaseScraper) SetProgress(progress int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	b.progress = progress
}

// IsStopped checks if the scraper has been stopped
func (b *BaseScraper) IsStopped() bool {
	return b.stopped
}

// Stop stops the scraper
func (b *BaseScraper) Stop() error {
	b.stopped = true
	return nil
}

// GetHTTPClient returns an HTTP client configured for VPN usage
func (b *BaseScraper) GetHTTPClient(ctx context.Context) (HTTPClient, error) {
	return b.vpnManager.GetHTTPClient(ctx)
}

// VPNManager defines the interface for VPN management
type VPNManager interface {
	// GetHTTPClient returns an HTTP client configured to use an optimal VPN
	GetHTTPClient(ctx context.Context) (HTTPClient, error)

	// MarkVPNFailed marks a VPN as having failed
	MarkVPNFailed(vpnID int64) error

	// MarkVPNSuccess marks a VPN as having succeeded
	MarkVPNSuccess(vpnID int64, bytesTransferred int64) error
}

// HTTPClient is an interface for making HTTP requests
type HTTPClient interface {
	Get(url string) ([]byte, error)
	GetStream(url string) ([]byte, error)
}

// DownloadResult represents the result of downloading media
type DownloadResult struct {
	Media *models.Media
	Error error
}

// ProcessMedia downloads and stores a media item
func (b *BaseScraper) ProcessMedia(ctx context.Context, media *models.Media) error {
	// Check if already exists
	exists, err := b.db.MediaExists(media.Hash)
	if err != nil {
		return fmt.Errorf("failed to check if media exists: %w", err)
	}
	if exists {
		return nil // Skip already downloaded media
	}

	// Download the media
	client, err := b.vpnManager.GetHTTPClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get HTTP client: %w", err)
	}

	data, err := client.GetStream(media.URL)
	if err != nil {
		return fmt.Errorf("failed to download media: %w", err)
	}

	// Save to file system
	localPath, err := b.fileStorage.SaveMedia(media, data)
	if err != nil {
		return fmt.Errorf("failed to save media: %w", err)
	}
	media.LocalPath = localPath

	// Save to database
	if err := b.db.InsertMedia(media); err != nil {
		return fmt.Errorf("failed to insert media into database: %w", err)
	}

	return nil
}
