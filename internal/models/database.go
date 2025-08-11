package models

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	sqlitecloud "github.com/sqlitecloud/sqlitecloud-go"
)

// Database represents the database connection and operations
type Database struct {
	db *sqlitecloud.SQCloud
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	log.Printf("Connecting to SQLite Cloud database: %s", maskConnectionString(dbPath))

	// Connect to SQLite Cloud
	db, err := sqlitecloud.Connect(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite Cloud: %v", err)
	}

	database := &Database{
		db: db,
	}

	// Create tables if they don't exist
	if err := database.createTables(); err != nil {
		return nil, err
	}

	return database, nil
}

// maskConnectionString hides the API key in logs for security
func maskConnectionString(connStr string) string {
	if strings.Contains(connStr, "apikey=") {
		parts := strings.Split(connStr, "apikey=")
		if len(parts) > 1 {
			return parts[0] + "apikey=***"
		}
	}
	return connStr
}

// executeSQL executes a SQL command using SQLite Cloud
func (d *Database) executeSQL(sql string, args ...interface{}) error {
	// Use SQLite Cloud's Execute method for DDL/DML operations
	if len(args) > 0 {
		return d.db.ExecuteArray(sql, args)
	}
	return d.db.Execute(sql)
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

	sql := `INSERT INTO channel_analytics (channel_id, channel_name, analytics_data)
			VALUES (?, ?, ?)`

	return d.db.ExecuteArray(sql, []interface{}{channelID, channelName, string(data)})
}

// StoreTrends stores channel trends data
func (d *Database) StoreTrends(channelID, channelName string, trends *ChannelTrends) error {
	data, err := json.Marshal(trends)
	if err != nil {
		return err
	}

	sql := `INSERT INTO channel_trends (channel_id, channel_name, trends_data)
			VALUES (?, ?, ?)`

	return d.db.ExecuteArray(sql, []interface{}{channelID, channelName, string(data)})
}

// GetLatestAnalytics retrieves the latest analytics for a channel
func (d *Database) GetLatestAnalytics(channelID string) (*ChannelAnalytics, error) {
	sql := `SELECT analytics_data FROM channel_analytics 
			WHERE channel_id = ? 
			ORDER BY created_at DESC LIMIT 1`

	result, err := d.db.SelectArray(sql, []interface{}{channelID})
	if err != nil {
		return nil, err
	}

	if result.GetNumberOfRows() == 0 {
		return nil, fmt.Errorf("no analytics found for channel %s", channelID)
	}

	analyticsData, err := result.GetStringValue(0, 0)
	if err != nil {
		return nil, err
	}

	var analytics ChannelAnalytics
	if err := json.Unmarshal([]byte(analyticsData), &analytics); err != nil {
		return nil, err
	}
	return &analytics, nil
}

// GetLatestTrends retrieves the latest trends for a channel
func (d *Database) GetLatestTrends(channelID string) (*ChannelTrends, error) {
	sql := `SELECT trends_data FROM channel_trends 
			WHERE channel_id = ? 
			ORDER BY created_at DESC LIMIT 1`

	result, err := d.db.SelectArray(sql, []interface{}{channelID})
	if err != nil {
		return nil, err
	}

	if result.GetNumberOfRows() == 0 {
		return nil, fmt.Errorf("no trends found for channel %s", channelID)
	}

	trendsData, err := result.GetStringValue(0, 0)
	if err != nil {
		return nil, err
	}

	var trends ChannelTrends
	if err := json.Unmarshal([]byte(trendsData), &trends); err != nil {
		return nil, err
	}
	return &trends, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}
