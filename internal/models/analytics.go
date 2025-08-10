package models

import "time"

// ChannelAnalytics represents engagement analytics for a channel
type ChannelAnalytics struct {
	ChannelID          string    `json:"channelId"`
	ChannelTitle       string    `json:"channelTitle"`
	ChannelName        string    `json:"channelName"`
	SubscriberCount    int64     `json:"subscriberCount"`
	ViewCount          int64     `json:"viewCount"`
	VideoCount         int64     `json:"videoCount"`
	TotalVideos        int       `json:"totalVideos"`
	AverageViews       float64   `json:"averageViews"`
	LikeToViewRatio    float64   `json:"likeToViewRatio"`
	CommentToViewRatio float64   `json:"commentToViewRatio"`
	TopEngagingVideos  []Video   `json:"topEngagingVideos"`
	TimeRange          TimeRange `json:"timeRange"`
	Timestamp          time.Time `json:"timestamp"`
}

// TimeRange represents the time period for analytics
type TimeRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// EngagementScore calculates the engagement score for a video
// This is a weighted combination of views, likes, and comments
func (v *Video) EngagementScore() float64 {
	if v.Views == 0 {
		return 0
	}

	// Weight factors (can be adjusted)
	const (
		viewWeight    = 1.0
		likeWeight    = 2.0
		commentWeight = 3.0
	)

	// Calculate weighted score
	score := (viewWeight * float64(v.Views)) +
		(likeWeight * float64(v.Likes)) +
		(commentWeight * float64(v.Comments))

	return score
}

// ChannelTrends represents trends and time series analytics for a channel
// Used for /channel/:id/trends endpoint

type VideoPerformancePoint struct {
	UploadDate      time.Time `json:"uploadDate"`
	Views           int64     `json:"views"`
	Likes           int64     `json:"likes"`
	Comments        int64     `json:"comments"`
	LikesToViews    float64   `json:"likesToViews"`
	CommentsToViews float64   `json:"commentsToViews"`
}

type RollingAverage struct {
	UploadIndex  int     `json:"uploadIndex"`
	AverageViews float64 `json:"averageViews"`
}

type UploadFrequency struct {
	Period string `json:"period"` // e.g. "week", "month"
	Count  int    `json:"count"`
}

type EngagementTrend struct {
	Period          string  `json:"period"`
	AverageViews    float64 `json:"averageViews"`
	AverageLikes    float64 `json:"averageLikes"`
	AverageComments float64 `json:"averageComments"`
}

type ChannelTrends struct {
	ChannelID               string                  `json:"channelId"`
	ChannelTitle            string                  `json:"channelTitle"`
	ChannelName             string                  `json:"channelName"`
	TrendingVideos          []Video                 `json:"trendingVideos"`
	PerformanceOverTime     []VideoPerformancePoint `json:"performanceOverTime"`
	RollingAverages         []RollingAverage        `json:"rollingAverages"`
	UploadFrequencyWeekly   []UploadFrequency       `json:"uploadFrequencyWeekly"`
	UploadFrequencyMonthly  []UploadFrequency       `json:"uploadFrequencyMonthly"`
	EngagementTrendsWeekly  []EngagementTrend       `json:"engagementTrendsWeekly"`
	EngagementTrendsMonthly []EngagementTrend       `json:"engagementTrendsMonthly"`
	Timestamp               time.Time               `json:"timestamp"`
}
