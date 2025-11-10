package tags

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/janderland/scraper/internal/models"
	"github.com/janderland/scraper/internal/storage"
)

// Tagger handles automatic tagging of media based on regex rules
type Tagger struct {
	db *storage.Database
}

// NewTagger creates a new tagger
func NewTagger(db *storage.Database) *Tagger {
	return &Tagger{
		db: db,
	}
}

// ApplyAutoTags applies automatic tags to a media item based on regex rules
func (t *Tagger) ApplyAutoTags(media *models.Media) error {
	// Get all tags with regex rules
	tags, err := t.db.GetAllTags()
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}

	// Build searchable text from media metadata
	searchText := t.buildSearchText(media)

	// Check each tag's regex rule
	for _, tag := range tags {
		if tag.RegexRule == "" {
			continue
		}

		matched, err := t.matchesRule(searchText, tag.RegexRule)
		if err != nil {
			fmt.Printf("Error matching regex for tag %s: %v\n", tag.Name, err)
			continue
		}

		if matched {
			// Add tag to media (auto = true)
			if err := t.db.AddMediaTag(media.ID, tag.ID, true); err != nil {
				fmt.Printf("Error adding tag %s to media %d: %v\n", tag.Name, media.ID, err)
				continue
			}
		}
	}

	return nil
}

// ApplyAutoTagsToAll applies automatic tags to all media items in the database
func (t *Tagger) ApplyAutoTagsToAll() error {
	// Get total count
	count, err := t.db.CountMedia()
	if err != nil {
		return fmt.Errorf("failed to count media: %w", err)
	}

	if count == 0 {
		return nil // No media to process
	}

	// Process in batches to avoid loading everything into memory
	batchSize := 100
	processed := 0
	errors := 0

	for offset := 0; offset < count; offset += batchSize {
		media, err := t.db.GetAllMedia(batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch media batch: %w", err)
		}

		for _, m := range media {
			if err := t.ApplyAutoTags(m); err != nil {
				fmt.Printf("Error applying auto-tags to media %d: %v\n", m.ID, err)
				errors++
			} else {
				processed++
			}
		}
	}

	if errors > 0 {
		fmt.Printf("Applied auto-tags to %d media items with %d errors\n", processed, errors)
	}

	return nil
}

// buildSearchText builds a searchable text string from media metadata
func (t *Tagger) buildSearchText(media *models.Media) string {
	var parts []string

	// Add basic fields
	if media.Title != "" {
		parts = append(parts, media.Title)
	}
	if media.Description != "" {
		parts = append(parts, media.Description)
	}
	if media.Author != "" {
		parts = append(parts, media.Author)
	}
	parts = append(parts, string(media.Platform))
	parts = append(parts, string(media.MediaType))

	// Add metadata values
	for key, value := range media.Metadata {
		parts = append(parts, fmt.Sprintf("%s:%v", key, value))
	}

	return strings.ToLower(strings.Join(parts, " "))
}

// matchesRule checks if text matches a regex rule
func (t *Tagger) matchesRule(text, rule string) (bool, error) {
	re, err := regexp.Compile(rule)
	if err != nil {
		return false, fmt.Errorf("invalid regex: %w", err)
	}

	return re.MatchString(text), nil
}

// CreateTag creates a new tag
func (t *Tagger) CreateTag(name, regexRule string) (*models.Tag, error) {
	// Validate regex if provided
	if regexRule != "" {
		if _, err := regexp.Compile(regexRule); err != nil {
			return nil, fmt.Errorf("invalid regex rule: %w", err)
		}
	}

	tag := &models.Tag{
		Name:      name,
		RegexRule: regexRule,
	}

	if err := t.db.InsertTag(tag); err != nil {
		return nil, fmt.Errorf("failed to insert tag: %w", err)
	}

	return tag, nil
}

// AddManualTag adds a tag to a media item manually
func (t *Tagger) AddManualTag(mediaID, tagID int64) error {
	return t.db.AddMediaTag(mediaID, tagID, false)
}

// GetMediaTags retrieves all tags for a media item
func (t *Tagger) GetMediaTags(mediaID int64) ([]*models.Tag, error) {
	return t.db.GetMediaTags(mediaID)
}

// GetAllTags retrieves all tags
func (t *Tagger) GetAllTags() ([]*models.Tag, error) {
	return t.db.GetAllTags()
}

// SuggestTags suggests tags based on common patterns in metadata
func (t *Tagger) SuggestTags(searchText string) ([]string, error) {
	// Common tag patterns
	patterns := map[string]string{
		"nsfw":        `(?i)(nsfw|nude|xxx)`,
		"funny":       `(?i)(funny|humor|lol|meme)`,
		"cute":        `(?i)(cute|aww|adorable)`,
		"nature":      `(?i)(nature|landscape|wildlife)`,
		"art":         `(?i)(art|drawing|painting)`,
		"photography": `(?i)(photo|photography|photographer)`,
		"video":       `(?i)(video|clip|footage)`,
		"gif":         `(?i)(gif|animated)`,
	}

	var suggestions []string
	lowerText := strings.ToLower(searchText)

	for tag, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(lowerText) {
			suggestions = append(suggestions, tag)
		}
	}

	return suggestions, nil
}
