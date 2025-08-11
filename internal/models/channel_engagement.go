package models

import (
	"encoding/json"
	"fmt"
	"log"
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
	checkSQL := `SELECT COUNT(*) FROM channel_engagement 
				 WHERE channel_id = ? AND engagement_type = ?`

	result, err := d.db.SelectArray(checkSQL, []interface{}{engagement.ChannelID, string(engagement.EngagementType)})
	if err != nil {
		log.Printf("Error checking existing record: %v", err)
		return fmt.Errorf("failed to check existing record: %v", err)
	}

	count, err := result.GetInt64Value(0, 0)
	if err != nil {
		log.Printf("Error getting count: %v", err)
		return fmt.Errorf("failed to get count: %v", err)
	}

	// Execute INSERT or UPDATE based on whether record exists
	var sql string
	var args []interface{}

	if count > 0 {
		log.Printf("Updating existing record for channel %s and type %s", engagement.ChannelID, engagement.EngagementType)
		sql = `UPDATE channel_engagement 
			   SET json_response = ?, update_date = CURRENT_TIMESTAMP
			   WHERE channel_id = ? AND engagement_type = ?`
		args = []interface{}{string(engagement.JSONResponse), engagement.ChannelID, string(engagement.EngagementType)}
	} else {
		log.Printf("Inserting new record for channel %s and type %s", engagement.ChannelID, engagement.EngagementType)
		sql = `INSERT INTO channel_engagement 
			   (channel_id, engagement_type, json_response) 
			   VALUES (?, ?, ?)`
		args = []interface{}{engagement.ChannelID, string(engagement.EngagementType), string(engagement.JSONResponse)}
	}

	err = d.db.ExecuteArray(sql, args)
	if err != nil {
		log.Printf("Error storing engagement: %v", err)
		return fmt.Errorf("failed to store engagement: %v", err)
	}

	log.Printf("Successfully stored engagement")
	return nil
}

// GetLatestEngagement retrieves the latest engagement record for a channel and type
func (d *Database) GetLatestEngagement(channelID string, engagementType EngagementType) (*ChannelEngagement, error) {
	sql := `SELECT id, channel_id, engagement_type, create_date, update_date, json_response 
			FROM channel_engagement 
			WHERE channel_id = ? AND engagement_type = ?
			ORDER BY create_date DESC LIMIT 1`

	result, err := d.db.SelectArray(sql, []interface{}{channelID, string(engagementType)})
	if err != nil {
		return nil, fmt.Errorf("failed to get latest engagement: %v", err)
	}

	if result.GetNumberOfRows() == 0 {
		return nil, nil // No record found
	}

	// Parse the fields
	id, err := result.GetInt64Value(0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse id: %v", err)
	}

	channelIDValue, err := result.GetStringValue(0, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel_id: %v", err)
	}

	engagementTypeValue, err := result.GetStringValue(0, 2)
	if err != nil {
		return nil, fmt.Errorf("failed to parse engagement_type: %v", err)
	}

	createDateStr, err := result.GetStringValue(0, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to parse create_date: %v", err)
	}

	updateDateStr, err := result.GetStringValue(0, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to parse update_date: %v", err)
	}

	jsonResponse, err := result.GetStringValue(0, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json_response: %v", err)
	}

	createDate, err := time.Parse("2006-01-02 15:04:05", createDateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse create_date: %v", err)
	}

	updateDate, err := time.Parse("2006-01-02 15:04:05", updateDateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse update_date: %v", err)
	}

	engagement := &ChannelEngagement{
		ID:             id,
		ChannelID:      channelIDValue,
		EngagementType: EngagementType(engagementTypeValue),
		CreateDate:     createDate,
		UpdateDate:     updateDate,
		JSONResponse:   json.RawMessage(jsonResponse),
	}

	return engagement, nil
}

// UpdateEngagement updates an existing engagement record
func (d *Database) UpdateEngagement(engagement *ChannelEngagement) error {
	sql := `UPDATE channel_engagement 
			SET json_response = ?, update_date = CURRENT_TIMESTAMP 
			WHERE channel_id = ? AND engagement_type = ?`

	return d.db.ExecuteArray(sql, []interface{}{string(engagement.JSONResponse), engagement.ChannelID, string(engagement.EngagementType)})
}

// GetEngagementHistory retrieves the engagement history for a channel and type
func (d *Database) GetEngagementHistory(channelID string, engagementType EngagementType, limit int) ([]*ChannelEngagement, error) {
	sql := `SELECT id, channel_id, engagement_type, create_date, update_date, json_response 
			FROM channel_engagement 
			WHERE channel_id = ? AND engagement_type = ?
			ORDER BY create_date DESC LIMIT ?`

	result, err := d.db.SelectArray(sql, []interface{}{channelID, string(engagementType), limit})
	if err != nil {
		return nil, err
	}

	var engagements []*ChannelEngagement
	rowCount := result.GetNumberOfRows()

	for r := uint64(0); r < rowCount; r++ {
		id, _ := result.GetInt64Value(r, 0)
		channelIDValue, _ := result.GetStringValue(r, 1)
		engagementTypeValue, _ := result.GetStringValue(r, 2)
		createDateStr, _ := result.GetStringValue(r, 3)
		updateDateStr, _ := result.GetStringValue(r, 4)
		jsonResponse, _ := result.GetStringValue(r, 5)

		createDate, _ := time.Parse("2006-01-02 15:04:05", createDateStr)
		updateDate, _ := time.Parse("2006-01-02 15:04:05", updateDateStr)

		engagement := &ChannelEngagement{
			ID:             id,
			ChannelID:      channelIDValue,
			EngagementType: EngagementType(engagementTypeValue),
			CreateDate:     createDate,
			UpdateDate:     updateDate,
			JSONResponse:   json.RawMessage(jsonResponse),
		}
		engagements = append(engagements, engagement)
	}

	return engagements, nil
}
