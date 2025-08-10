package models

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// EngagementType represents the type of engagement data
type EngagementType string

const (
	EngagementTypeAnalytics EngagementType = "analytics"
	EngagementTypeTrends    EngagementType = "trends"
)

// ChannelEngagement represents a record in the channel_engagement table
type ChannelEngagement struct {
	ID             int64           `json:"id"`
	ChannelID      string          `json:"channel_id"`
	EngagementType EngagementType  `json:"engagement_type"`
	CreateDate     time.Time       `json:"create_date"`
	UpdateDate     time.Time       `json:"update_date"`
	JSONResponse   json.RawMessage `json:"json_response"`
}

// CreateChannelEngagementTable creates the channel_engagement table if it doesn't exist
func (d *Database) CreateChannelEngagementTable() error {
	sql := `
	CREATE TABLE IF NOT EXISTS channel_engagement (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id TEXT NOT NULL,
		engagement_type TEXT NOT NULL CHECK(engagement_type IN ('analytics', 'trends')),
		create_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		update_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		json_response JSON NOT NULL,
		UNIQUE(channel_id, engagement_type)
	);
	CREATE INDEX IF NOT EXISTS idx_channel_engagement_channel_id ON channel_engagement(channel_id);
	CREATE INDEX IF NOT EXISTS idx_channel_engagement_type ON channel_engagement(engagement_type);
	`
	return d.executeSQL(sql)
}

// StoreEngagement stores a new engagement record
func (d *Database) StoreEngagement(engagement *ChannelEngagement) error {
	log.Printf("Storing engagement for channel %s, type %s", engagement.ChannelID, engagement.EngagementType)

	// First check if a record exists with both channel_id AND engagement_type
	checkSQL := fmt.Sprintf(`SELECT COUNT(*) FROM channel_engagement 
		WHERE channel_id = '%s' AND engagement_type = '%s'`,
		engagement.ChannelID, string(engagement.EngagementType))

	cmd := exec.Command(d.sqlitePath, d.dbPath, checkSQL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error checking existing record: %v, Output: %s", err, string(output))
		return fmt.Errorf("failed to check existing record: %v", err)
	}

	// Convert output to string and trim whitespace
	countStr := strings.TrimSpace(string(output))
	count, err := strconv.Atoi(countStr)
	if err != nil {
		log.Printf("Error parsing count: %v", err)
		return fmt.Errorf("failed to parse count: %v", err)
	}

	// Create a temporary file for the SQL command
	tmpFile, err := os.CreateTemp("", "sqlite-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Escape the JSON response
	escapedJSON := strings.ReplaceAll(string(engagement.JSONResponse), "'", "''")

	// Write the SQL command to the temporary file
	var sql string
	if count > 0 {
		log.Printf("Updating existing record for channel %s and type %s", engagement.ChannelID, engagement.EngagementType)
		sql = fmt.Sprintf(`UPDATE channel_engagement 
			SET json_response = '%s', update_date = CURRENT_TIMESTAMP
			WHERE channel_id = '%s' AND engagement_type = '%s';`,
			escapedJSON,
			engagement.ChannelID,
			string(engagement.EngagementType))
	} else {
		log.Printf("Inserting new record for channel %s and type %s", engagement.ChannelID, engagement.EngagementType)
		sql = fmt.Sprintf(`INSERT INTO channel_engagement 
			(channel_id, engagement_type, json_response) 
			VALUES ('%s', '%s', '%s');`,
			engagement.ChannelID,
			string(engagement.EngagementType),
			escapedJSON)
	}

	if _, err := tmpFile.WriteString(sql); err != nil {
		return fmt.Errorf("failed to write SQL to temporary file: %v", err)
	}
	tmpFile.Close()

	// Execute the SQL command using the temporary file
	cmd = exec.Command(d.sqlitePath, d.dbPath, ".read "+tmpFile.Name())
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error storing engagement: %v, Output: %s", err, string(output))
		return fmt.Errorf("failed to store engagement: %v, Output: %s", err, string(output))
	}

	log.Printf("Successfully stored engagement")
	return nil
}

// GetLatestEngagement retrieves the latest engagement record for a channel and type
func (d *Database) GetLatestEngagement(channelID string, engagementType EngagementType) (*ChannelEngagement, error) {
	sql := fmt.Sprintf(`SELECT id, channel_id, engagement_type, create_date, update_date, json_response 
		FROM channel_engagement 
		WHERE channel_id = '%s' AND engagement_type = '%s'
		ORDER BY create_date DESC LIMIT 1`,
		channelID, engagementType)

	cmd := exec.Command(d.sqlitePath, d.dbPath, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If no rows found, return nil without error
		if strings.Contains(string(output), "no such table") || strings.Contains(string(output), "no rows") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest engagement: %v, Output: %s", err, string(output))
	}

	// Parse the output line by line
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return nil, nil
	}

	// Parse the first line which contains our data
	fields := strings.Split(lines[0], "|")
	if len(fields) != 6 {
		return nil, fmt.Errorf("invalid number of fields in output: %d", len(fields))
	}

	// Parse the fields
	id, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse id: %v", err)
	}

	createDate, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(fields[3]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse create_date: %v", err)
	}

	updateDate, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(fields[4]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse update_date: %v", err)
	}

	engagement := &ChannelEngagement{
		ID:             id,
		ChannelID:      strings.TrimSpace(fields[1]),
		EngagementType: EngagementType(strings.TrimSpace(fields[2])),
		CreateDate:     createDate,
		UpdateDate:     updateDate,
		JSONResponse:   json.RawMessage(strings.TrimSpace(fields[5])),
	}

	return engagement, nil
}

// UpdateEngagement updates an existing engagement record
func (d *Database) UpdateEngagement(engagement *ChannelEngagement) error {
	sql := `UPDATE channel_engagement 
		SET json_response = ?, update_date = CURRENT_TIMESTAMP 
		WHERE channel_id = ? AND engagement_type = ?`

	cmd := exec.Command(d.sqlitePath, d.dbPath, sql,
		string(engagement.JSONResponse),
		engagement.ChannelID,
		string(engagement.EngagementType))

	return cmd.Run()
}

// GetEngagementHistory retrieves the engagement history for a channel and type
func (d *Database) GetEngagementHistory(channelID string, engagementType EngagementType, limit int) ([]*ChannelEngagement, error) {
	sql := fmt.Sprintf(`SELECT id, channel_id, engagement_type, create_date, update_date, json_response 
		FROM channel_engagement 
		WHERE channel_id = '%s' AND engagement_type = '%s'
		ORDER BY create_date DESC LIMIT %d`,
		channelID, engagementType, limit)

	cmd := exec.Command(d.sqlitePath, d.dbPath, sql)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var engagements []*ChannelEngagement
	if err := json.Unmarshal(output, &engagements); err != nil {
		return nil, err
	}
	return engagements, nil
}
