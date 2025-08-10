package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrMissingAPIKey = errors.New("YouTube API key is required")
)

// Config holds the application configuration
type Config struct {
	YouTubeAPIKey string
	DBPath        string
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	// Get YouTube API key from environment
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY environment variable is required")
	}

	// Get database path from environment or use default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		// Use the pre-compiled SQLite setup path
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %v", err)
		}
		dbPath = filepath.Join(wd, "..", "sqlite", "yt_insights.db")
	}

	return &Config{
		YouTubeAPIKey: apiKey,
		DBPath:        dbPath,
	}, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.YouTubeAPIKey == "" {
		return fmt.Errorf("%w: YOUTUBE_API_KEY environment variable is not set", ErrMissingAPIKey)
	}
	return nil
}
