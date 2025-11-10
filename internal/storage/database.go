package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/janderland/scraper/internal/models"
)

// Database handles all database operations
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection and initializes tables
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	d := &Database{db: db}
	if err := d.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return d, nil
}

// createTables creates all necessary database tables
func (d *Database) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		media_type TEXT NOT NULL,
		url TEXT NOT NULL,
		local_path TEXT NOT NULL UNIQUE,
		hash TEXT NOT NULL UNIQUE,
		title TEXT,
		description TEXT,
		author TEXT,
		source_url TEXT,
		upvotes INTEGER DEFAULT 0,
		posted_at DATETIME,
		downloaded_at DATETIME NOT NULL,
		metadata TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_media_hash ON media(hash);
	CREATE INDEX IF NOT EXISTS idx_media_platform ON media(platform);
	CREATE INDEX IF NOT EXISTS idx_media_author ON media(author);
	CREATE INDEX IF NOT EXISTS idx_media_posted_at ON media(posted_at);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		regex_rule TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS media_tags (
		media_id INTEGER NOT NULL,
		tag_id INTEGER NOT NULL,
		auto BOOLEAN DEFAULT 0,
		PRIMARY KEY (media_id, tag_id),
		FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
		FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS vpn_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		protocol TEXT NOT NULL,
		username TEXT,
		password TEXT,
		config_path TEXT,
		max_bandwidth INTEGER DEFAULT 0,
		active BOOLEAN DEFAULT 1,
		failure_count INTEGER DEFAULT 0,
		last_used DATETIME
	);

	CREATE TABLE IF NOT EXISTS download_stats (
		vpn_id INTEGER PRIMARY KEY,
		bytes_transferred INTEGER DEFAULT 0,
		request_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		avg_latency INTEGER DEFAULT 0,
		last_updated DATETIME NOT NULL,
		FOREIGN KEY (vpn_id) REFERENCES vpn_configs(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS scraper_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		filter TEXT NOT NULL,
		status TEXT NOT NULL,
		progress INTEGER DEFAULT 0,
		started_at DATETIME,
		completed_at DATETIME,
		error TEXT
	);
	`

	_, err := d.db.Exec(schema)
	return err
}

// MediaExists checks if a media item already exists by hash
func (d *Database) MediaExists(hash string) (bool, error) {
	var exists bool
	err := d.db.QueryRow("SELECT EXISTS(SELECT 1 FROM media WHERE hash = ?)", hash).Scan(&exists)
	return exists, err
}

// InsertMedia inserts a new media item
func (d *Database) InsertMedia(media *models.Media) error {
	metadataJSON, err := json.Marshal(media.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	result, err := d.db.Exec(`
		INSERT INTO media (platform, media_type, url, local_path, hash, title, description,
			author, source_url, upvotes, posted_at, downloaded_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		media.Platform, media.MediaType, media.URL, media.LocalPath, media.Hash,
		media.Title, media.Description, media.Author, media.SourceURL, media.Upvotes,
		media.PostedAt, media.DownloadedAt, metadataJSON,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	media.ID = id
	return nil
}

// GetMediaByID retrieves a media item by ID
func (d *Database) GetMediaByID(id int64) (*models.Media, error) {
	media := &models.Media{}
	var metadataJSON string
	var postedAt sql.NullTime

	err := d.db.QueryRow(`
		SELECT id, platform, media_type, url, local_path, hash, title, description,
			author, source_url, upvotes, posted_at, downloaded_at, metadata
		FROM media WHERE id = ?`, id).Scan(
		&media.ID, &media.Platform, &media.MediaType, &media.URL, &media.LocalPath,
		&media.Hash, &media.Title, &media.Description, &media.Author, &media.SourceURL,
		&media.Upvotes, &postedAt, &media.DownloadedAt, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	if postedAt.Valid {
		media.PostedAt = postedAt.Time
	}

	if err := json.Unmarshal([]byte(metadataJSON), &media.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return media, nil
}

// SearchMedia searches for media items with filters
func (d *Database) SearchMedia(platform models.Platform, searchTerm string, tags []int64, limit, offset int) ([]*models.Media, error) {
	query := `
		SELECT DISTINCT m.id, m.platform, m.media_type, m.url, m.local_path, m.hash,
			m.title, m.description, m.author, m.source_url, m.upvotes,
			m.posted_at, m.downloaded_at, m.metadata
		FROM media m
		LEFT JOIN media_tags mt ON m.id = mt.media_id
		WHERE 1=1
	`
	args := []interface{}{}

	if platform != "" {
		query += " AND m.platform = ?"
		args = append(args, platform)
	}

	if searchTerm != "" {
		query += " AND (m.title LIKE ? OR m.description LIKE ? OR m.author LIKE ?)"
		searchPattern := "%" + searchTerm + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	if len(tags) > 0 {
		query += " AND mt.tag_id IN ("
		for i := range tags {
			if i > 0 {
				query += ","
			}
			query += "?"
			args = append(args, tags[i])
		}
		query += ")"
	}

	query += " ORDER BY m.downloaded_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.Media
	for rows.Next() {
		media := &models.Media{}
		var metadataJSON string
		var postedAt sql.NullTime

		err := rows.Scan(
			&media.ID, &media.Platform, &media.MediaType, &media.URL, &media.LocalPath,
			&media.Hash, &media.Title, &media.Description, &media.Author, &media.SourceURL,
			&media.Upvotes, &postedAt, &media.DownloadedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if postedAt.Valid {
			media.PostedAt = postedAt.Time
		}

		if err := json.Unmarshal([]byte(metadataJSON), &media.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, media)
	}

	return results, rows.Err()
}

// InsertTag creates a new tag
func (d *Database) InsertTag(tag *models.Tag) error {
	result, err := d.db.Exec(`
		INSERT INTO tags (name, regex_rule, created_at)
		VALUES (?, ?, ?)`,
		tag.Name, tag.RegexRule, time.Now(),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	tag.ID = id
	return nil
}

// GetAllTags retrieves all tags
func (d *Database) GetAllTags() ([]*models.Tag, error) {
	rows, err := d.db.Query("SELECT id, name, regex_rule, created_at FROM tags ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		tag := &models.Tag{}
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.RegexRule, &tag.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// AddMediaTag adds a tag to a media item
func (d *Database) AddMediaTag(mediaID, tagID int64, auto bool) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO media_tags (media_id, tag_id, auto)
		VALUES (?, ?, ?)`,
		mediaID, tagID, auto,
	)
	return err
}

// GetMediaTags retrieves all tags for a media item
func (d *Database) GetMediaTags(mediaID int64) ([]*models.Tag, error) {
	rows, err := d.db.Query(`
		SELECT t.id, t.name, t.regex_rule, t.created_at
		FROM tags t
		JOIN media_tags mt ON t.id = mt.tag_id
		WHERE mt.media_id = ?`,
		mediaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		tag := &models.Tag{}
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.RegexRule, &tag.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetAllMedia retrieves all media items from the database
func (d *Database) GetAllMedia(limit, offset int) ([]*models.Media, error) {
	query := `
		SELECT id, platform, media_type, url, local_path, hash, title, description,
			author, source_url, upvotes, posted_at, downloaded_at, metadata
		FROM media
		ORDER BY downloaded_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := d.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.Media
	for rows.Next() {
		media := &models.Media{}
		var metadataJSON string
		var postedAt sql.NullTime

		err := rows.Scan(
			&media.ID, &media.Platform, &media.MediaType, &media.URL, &media.LocalPath,
			&media.Hash, &media.Title, &media.Description, &media.Author, &media.SourceURL,
			&media.Upvotes, &postedAt, &media.DownloadedAt, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if postedAt.Valid {
			media.PostedAt = postedAt.Time
		}

		if err := json.Unmarshal([]byte(metadataJSON), &media.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		results = append(results, media)
	}

	return results, rows.Err()
}

// CountMedia returns the total number of media items
func (d *Database) CountMedia() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM media").Scan(&count)
	return count, err
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}
