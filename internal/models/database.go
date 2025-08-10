package models

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Database represents the database connection and operations
type Database struct {
	dbPath     string
	sqlitePath string
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Construct the path to the SQLite executable
	sqlitePath := filepath.Join(wd, "..", "sqlite", "sqlite3.exe")

	// Check if SQLite executable exists
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SQLite executable not found at %s", sqlitePath)
	}

	db := &Database{
		dbPath:     dbPath,
		sqlitePath: sqlitePath,
	}

	// Create tables if they don't exist
	if err := db.createTables(); err != nil {
		return nil, err
	}

	return db, nil
}

// executeSQL executes a SQL command using the SQLite3 executable
func (d *Database) executeSQL(sql string, args ...interface{}) error {
	// Format the SQL command with arguments
	formattedSQL := fmt.Sprintf(sql, args...)

	// Create the command
	cmd := exec.Command(d.sqlitePath, d.dbPath, formattedSQL)

	// Execute the command
	return cmd.Run()
}

// createTables creates the necessary tables if they don't exist
func (d *Database) createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS channel_engagement (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id TEXT NOT NULL,
			engagement_type TEXT NOT NULL CHECK(engagement_type IN ('analytics', 'trends')),
			create_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			update_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			json_response TEXT NOT NULL,
			CONSTRAINT unique_channel_engagement UNIQUE(channel_id, engagement_type)
		)`,
		`CREATE TABLE IF NOT EXISTS channel_analytics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id TEXT NOT NULL,
			channel_name TEXT NOT NULL,
			analytics_data TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS channel_trends (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id TEXT NOT NULL,
			channel_name TEXT NOT NULL,
			trends_data TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if err := d.executeSQL(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}
	return nil
}

// StoreAnalytics stores channel analytics data
func (d *Database) StoreAnalytics(channelID, channelName string, analytics *ChannelAnalytics) error {
	data, err := json.Marshal(analytics)
	if err != nil {
		return err
	}

	return d.executeSQL(`
		INSERT INTO channel_analytics (channel_id, channel_name, analytics_data)
		VALUES (?, ?, ?)
	`, channelID, channelName, string(data))
}

// StoreTrends stores channel trends data
func (d *Database) StoreTrends(channelID, channelName string, trends *ChannelTrends) error {
	data, err := json.Marshal(trends)
	if err != nil {
		return err
	}

	return d.executeSQL(`
		INSERT INTO channel_trends (channel_id, channel_name, trends_data)
		VALUES (?, ?, ?)
	`, channelID, channelName, string(data))
}

// GetLatestAnalytics retrieves the latest analytics for a channel
func (d *Database) GetLatestAnalytics(channelID string) (*ChannelAnalytics, error) {
	sql := fmt.Sprintf(`SELECT analytics_data FROM channel_analytics 
		WHERE channel_id = '%s' 
		ORDER BY created_at DESC LIMIT 1`,
		channelID)

	cmd := exec.Command(d.sqlitePath, d.dbPath, sql)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var analytics ChannelAnalytics
	if err := json.Unmarshal(output, &analytics); err != nil {
		return nil, err
	}
	return &analytics, nil
}

// GetLatestTrends retrieves the latest trends for a channel
func (d *Database) GetLatestTrends(channelID string) (*ChannelTrends, error) {
	sql := fmt.Sprintf(`SELECT trends_data FROM channel_trends 
		WHERE channel_id = '%s' 
		ORDER BY created_at DESC LIMIT 1`,
		channelID)

	cmd := exec.Command(d.sqlitePath, d.dbPath, sql)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var trends ChannelTrends
	if err := json.Unmarshal(output, &trends); err != nil {
		return nil, err
	}
	return &trends, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	// No need to close anything as we're using the executable directly
	return nil
}
