package storage

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/janderland/scraper/internal/models"
)

// FileStorage handles file system operations for media
type FileStorage struct {
	basePath string
}

// NewFileStorage creates a new file storage manager
func NewFileStorage(basePath string) (*FileStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
	}, nil
}

// SaveMedia saves media data to the file system
func (fs *FileStorage) SaveMedia(media *models.Media, data []byte) (string, error) {
	// Calculate hash if not already set
	if media.Hash == "" {
		media.Hash = fs.calculateHash(data)
	}

	// Create directory structure: basePath/platform/year/month/
	now := time.Now()
	dirPath := filepath.Join(
		fs.basePath,
		string(media.Platform),
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
	)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Determine file extension
	ext := fs.getExtension(media.MediaType, media.URL)

	// Generate filename: hash + extension
	filename := fmt.Sprintf("%s%s", media.Hash[:16], ext)
	fullPath := filepath.Join(dirPath, filename)

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fullPath, nil
}

// GetMedia retrieves media data from the file system
func (fs *FileStorage) GetMedia(localPath string) ([]byte, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// DeleteMedia deletes media from the file system
func (fs *FileStorage) DeleteMedia(localPath string) error {
	if err := os.Remove(localPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// MediaExists checks if a media file exists
func (fs *FileStorage) MediaExists(localPath string) bool {
	_, err := os.Stat(localPath)
	return err == nil
}

// calculateHash calculates SHA256 hash of data
func (fs *FileStorage) calculateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// CalculateHash calculates hash from a reader
func (fs *FileStorage) CalculateHash(r io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// getExtension determines file extension based on media type and URL
func (fs *FileStorage) getExtension(mediaType models.MediaType, url string) string {
	// Try to extract from URL first
	ext := filepath.Ext(url)
	if ext != "" && len(ext) <= 5 {
		return ext
	}

	// Fallback to media type
	switch mediaType {
	case models.MediaTypeImage:
		return ".jpg"
	case models.MediaTypeVideo:
		return ".mp4"
	case models.MediaTypeGIF:
		return ".gif"
	default:
		return ".bin"
	}
}

// GetStorageStats returns storage statistics
func (fs *FileStorage) GetStorageStats() (totalSize int64, fileCount int, err error) {
	err = filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})
	return
}
