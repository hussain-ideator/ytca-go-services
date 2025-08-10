package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yt-insights/internal/models"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	youtubeAPIBaseURL = "https://www.googleapis.com/youtube/v3"
)

// YouTubeClient handles direct HTTP requests to YouTube API
type YouTubeClient struct {
	apiKey string
	client *http.Client
}

// NewYouTubeClient creates a new YouTube client
func NewYouTubeClient(apiKey string) *YouTubeClient {
	return &YouTubeClient{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// ExtractChannelIDFromURL extracts the channel ID from various YouTube URL formats
func (c *YouTubeClient) ExtractChannelIDFromURL(channelURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(channelURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Handle different URL formats
	switch {
	case strings.Contains(parsedURL.Host, "youtube.com"):
		// Handle youtube.com URLs
		path := parsedURL.Path
		if strings.HasPrefix(path, "/channel/") {
			// Format: youtube.com/channel/UC...
			return strings.TrimPrefix(path, "/channel/"), nil
		} else if strings.HasPrefix(path, "/c/") || strings.HasPrefix(path, "/user/") {
			// Format: youtube.com/c/ChannelName or youtube.com/user/Username
			// We need to make an API call to get the channel ID
			customURL := strings.TrimPrefix(path, "/c/")
			customURL = strings.TrimPrefix(customURL, "/user/")
			return c.getChannelIDFromCustomURL(customURL)
		} else if strings.HasPrefix(path, "/@") {
			// Format: youtube.com/@Handle
			handle := strings.TrimPrefix(path, "/@")
			return c.getChannelIDFromHandle(handle)
		}
	case strings.Contains(parsedURL.Host, "youtu.be"):
		// Handle youtu.be URLs (these are usually video URLs)
		return "", fmt.Errorf("youtu.be URLs are typically video URLs, not channel URLs")
	}

	return "", fmt.Errorf("unsupported YouTube URL format")
}

// getChannelIDFromCustomURL gets the channel ID from a custom URL or username
func (c *YouTubeClient) getChannelIDFromCustomURL(customURL string) (string, error) {
	url := fmt.Sprintf("%s/channels?part=id&forUsername=%s&key=%s",
		youtubeAPIBaseURL, customURL, c.apiKey)

	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch channel ID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube API returned status code: %d", resp.StatusCode)
	}

	var response struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("no channel found for URL: %s", customURL)
	}

	return response.Items[0].ID, nil
}

// getChannelIDFromHandle gets the channel ID from a channel handle
func (c *YouTubeClient) getChannelIDFromHandle(handle string) (string, error) {
	// First try to get the channel directly using the handle
	url := fmt.Sprintf("%s/channels?part=id,snippet&forHandle=%s&key=%s",
		youtubeAPIBaseURL, handle, c.apiKey)

	resp, err := c.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch channel ID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube API returned status code: %d", resp.StatusCode)
	}

	var response struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// If we found a channel with the exact handle, return it
	if len(response.Items) > 0 {
		fmt.Printf("Found channel: %s\n", response.Items[0].Snippet.Title)
		return response.Items[0].ID, nil
	}

	// If direct lookup failed, try search with exact handle
	searchURL := fmt.Sprintf("%s/search?part=snippet&q=@%s&type=channel&key=%s",
		youtubeAPIBaseURL, handle, c.apiKey)

	resp, err = c.client.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch channel ID: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube API returned status code: %d", resp.StatusCode)
	}

	var searchResponse struct {
		Items []struct {
			ID struct {
				ChannelID string `json:"channelId"`
			} `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(searchResponse.Items) == 0 {
		return "", fmt.Errorf("no channel found for handle: @%s", handle)
	}

	// Print all found channels for debugging
	fmt.Println("\nFound channels:")
	for i, item := range searchResponse.Items {
		fmt.Printf("%d. %s (ID: %s)\n", i+1, item.Snippet.Title, item.ID.ChannelID)
	}

	// Get the first result and verify it's the correct channel
	channelID := searchResponse.Items[0].ID.ChannelID
	channel, err := c.GetChannelByID(channelID)
	if err != nil {
		return "", fmt.Errorf("failed to verify channel: %w", err)
	}

	// Verify the channel handle matches
	if !strings.EqualFold(channel.Title, handle) {
		fmt.Printf("Warning: Found channel '%s' might not be the exact match for handle '@%s'\n",
			channel.Title, handle)
	}

	return channelID, nil
}

// SearchChannelByTitle searches for a channel by its title
func (c *YouTubeClient) SearchChannelByTitle(title string) (*models.Channel, error) {
	// First, search for the channel
	searchURL := fmt.Sprintf("%s/search?part=snippet&q=%s&type=channel&key=%s",
		youtubeAPIBaseURL, title, c.apiKey)

	resp, err := c.client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search for channel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YouTube API returned status code: %d", resp.StatusCode)
	}

	var searchResponse struct {
		Items []struct {
			ID struct {
				ChannelID string `json:"channelId"`
			} `json:"id"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	if len(searchResponse.Items) == 0 {
		return nil, fmt.Errorf("no channels found with title: %s", title)
	}

	// Get the first matching channel's ID and fetch its details
	channelID := searchResponse.Items[0].ID.ChannelID
	return c.GetChannelByID(channelID)
}

// GetChannelByID fetches channel information by channel ID
func (c *YouTubeClient) GetChannelByID(channelID string) (*models.Channel, error) {
	url := fmt.Sprintf("%s/channels?part=snippet,statistics&id=%s&key=%s",
		youtubeAPIBaseURL, channelID, c.apiKey)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("YouTube API returned status code: %d", resp.StatusCode)
	}

	var response models.ChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	item := response.Items[0]
	subscribers, _ := strconv.ParseInt(item.Statistics.SubscriberCount, 10, 64)
	views, _ := strconv.ParseInt(item.Statistics.ViewCount, 10, 64)
	videos, _ := strconv.ParseInt(item.Statistics.VideoCount, 10, 64)

	return &models.Channel{
		ID:          item.ID,
		Title:       item.Snippet.Title,
		Description: item.Snippet.Description,
		Subscribers: subscribers,
		ViewCount:   views,
		VideoCount:  videos,
		Thumbnail:   item.Snippet.Thumbnails.Default.URL,
	}, nil
}

// GetChannelVideos fetches videos for a channel with optional filtering
func (c *YouTubeClient) GetChannelVideos(channelID string, filter models.VideoFilter) ([]models.Video, error) {
	// First, get the uploads playlist ID for the channel
	channelURL := fmt.Sprintf("%s/channels?part=contentDetails&id=%s&key=%s",
		youtubeAPIBaseURL, channelID, c.apiKey)

	resp, err := c.client.Get(channelURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel details: %w", err)
	}
	defer resp.Body.Close()

	var channelResponse struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&channelResponse); err != nil {
		return nil, fmt.Errorf("failed to decode channel response: %w", err)
	}

	if len(channelResponse.Items) == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	uploadsPlaylistID := channelResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads

	// Get videos from the uploads playlist with pagination
	var allVideoIDs []string
	nextPageToken := ""
	maxResults := 50 // YouTube API maximum per request

	for {
		// Construct URL with pagination
		playlistURL := fmt.Sprintf("%s/playlistItems?part=snippet&playlistId=%s&maxResults=%d&key=%s",
			youtubeAPIBaseURL, uploadsPlaylistID, maxResults, c.apiKey)
		if nextPageToken != "" {
			playlistURL += "&pageToken=" + nextPageToken
		}

		resp, err = c.client.Get(playlistURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch playlist items: %w", err)
		}
		defer resp.Body.Close()

		var playlistResponse struct {
			Items []struct {
				Snippet struct {
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&playlistResponse); err != nil {
			return nil, fmt.Errorf("failed to decode playlist response: %w", err)
		}

		// Add video IDs from this page
		for _, item := range playlistResponse.Items {
			allVideoIDs = append(allVideoIDs, item.Snippet.ResourceID.VideoID)
		}

		// Check if we've reached the desired number of videos or there are no more pages
		if len(allVideoIDs) >= filter.MaxVideos || playlistResponse.NextPageToken == "" {
			break
		}

		nextPageToken = playlistResponse.NextPageToken
	}

	// Limit to requested number of videos
	if len(allVideoIDs) > filter.MaxVideos {
		allVideoIDs = allVideoIDs[:filter.MaxVideos]
	}

	// Get detailed video information in batches (YouTube API has a limit of 50 videos per request)
	var allVideos []models.Video
	for i := 0; i < len(allVideoIDs); i += 50 {
		end := i + 50
		if end > len(allVideoIDs) {
			end = len(allVideoIDs)
		}
		batch := allVideoIDs[i:end]

		videosURL := fmt.Sprintf("%s/videos?part=snippet,contentDetails,statistics&id=%s&key=%s",
			youtubeAPIBaseURL, strings.Join(batch, ","), c.apiKey)

		resp, err = c.client.Get(videosURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch video details: %w", err)
		}
		defer resp.Body.Close()

		var videoResponse models.VideoListResponse
		if err := json.NewDecoder(resp.Body).Decode(&videoResponse); err != nil {
			return nil, fmt.Errorf("failed to decode video response: %w", err)
		}

		// Convert response to Video objects
		for _, item := range videoResponse.Items {
			// For Go client: v.Statistics.ViewCount is uint64, v.Snippet.PublishedAt is string
			views := int64(0)
			likes := int64(0)
			comments := int64(0)
			publishedAt := time.Time{}
			if viewCount, ok := any(item.Statistics.ViewCount).(uint64); ok {
				views = int64(viewCount)
			}
			if likeCount, ok := any(item.Statistics.LikeCount).(uint64); ok {
				likes = int64(likeCount)
			}
			if commentCount, ok := any(item.Statistics.CommentCount).(uint64); ok {
				comments = int64(commentCount)
			}
			if pub, ok := any(item.Snippet.PublishedAt).(string); ok {
				parsed, err := time.Parse(time.RFC3339, pub)
				if err == nil {
					publishedAt = parsed
				}
			} else if pub, ok := any(item.Snippet.PublishedAt).(time.Time); ok {
				publishedAt = pub
			}

			// Apply filters
			if views < filter.MinViews || likes < filter.MinLikes {
				continue
			}

			allVideos = append(allVideos, models.Video{
				ID:           item.ID,
				Title:        item.Snippet.Title,
				Description:  item.Snippet.Description,
				Views:        views,
				Likes:        likes,
				Comments:     comments,
				UploadDate:   publishedAt,
				PublishedAt:  publishedAt,
				Duration:     item.ContentDetails.Duration,
				Thumbnail:    item.Snippet.Thumbnails.Default.URL,
				ViewCount:    views,
				LikeCount:    likes,
				CommentCount: comments,
			})
		}
	}

	// Sort videos based on filter
	switch filter.SortBy {
	case models.SortByViews:
		sort.Slice(allVideos, func(i, j int) bool {
			return allVideos[i].Views > allVideos[j].Views
		})
	case models.SortByLikes:
		sort.Slice(allVideos, func(i, j int) bool {
			return allVideos[i].Likes > allVideos[j].Likes
		})
	case models.SortByRecency:
		sort.Slice(allVideos, func(i, j int) bool {
			return allVideos[i].UploadDate.After(allVideos[j].UploadDate)
		})
	}

	return allVideos, nil
}

// GetChannelAnalytics calculates engagement analytics for a channel
func (c *YouTubeClient) GetChannelAnalytics(channelID string) (*models.ChannelAnalytics, error) {
	// Get channel info first
	channel, err := c.GetChannelByID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %w", err)
	}

	// Get all videos for the channel
	filter := models.VideoFilter{
		MaxVideos: int(channel.VideoCount), // Use actual video count from channel
		SortBy:    models.SortByRecency,
	}
	videos, err := c.GetChannelVideos(channelID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel videos: %w", err)
	}

	if len(videos) == 0 {
		return nil, fmt.Errorf("no videos found for channel")
	}

	// Calculate analytics
	var totalViews, totalLikes, totalComments int64
	for _, video := range videos {
		totalViews += video.Views
		totalLikes += video.Likes
		totalComments += video.Comments
	}

	// Calculate averages and ratios
	averageViews := float64(totalViews) / float64(len(videos))
	likeToViewRatio := float64(totalLikes) / float64(totalViews)
	commentToViewRatio := float64(totalComments) / float64(totalViews)

	// Sort videos by engagement score
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].EngagementScore() > videos[j].EngagementScore()
	})

	// Get top 5 most engaging videos
	topVideos := videos
	if len(videos) > 5 {
		topVideos = videos[:5]
	}

	// Get time range
	timeRange := models.TimeRange{
		StartDate: videos[len(videos)-1].UploadDate.Format("2006-01-02"),
		EndDate:   videos[0].UploadDate.Format("2006-01-02"),
	}

	return &models.ChannelAnalytics{
		ChannelID:          channelID,
		ChannelTitle:       channel.Title,
		TotalVideos:        len(videos),
		AverageViews:       averageViews,
		LikeToViewRatio:    likeToViewRatio,
		CommentToViewRatio: commentToViewRatio,
		TopEngagingVideos:  topVideos,
		TimeRange:          timeRange,
	}, nil
}

// GetChannelTrends computes trends and time series analytics for a channel
func (c *YouTubeClient) GetChannelTrends(channelID string) (*models.ChannelTrends, error) {
	// Get channel info
	channel, err := c.GetChannelByID(channelID)
	if err != nil {
		return nil, err
	}
	filter := models.VideoFilter{
		MaxVideos: int(channel.VideoCount),
		SortBy:    models.SortByRecency,
	}
	videos, err := c.GetChannelVideos(channelID, filter)
	if err != nil {
		return nil, err
	}
	if len(videos) == 0 {
		return nil, nil
	}

	// 1. Video performance over time
	var perf []models.VideoPerformancePoint
	for _, v := range videos {
		likesToViews := 0.0
		commentsToViews := 0.0
		if v.Views > 0 {
			likesToViews = float64(v.Likes) / float64(v.Views)
			commentsToViews = float64(v.Comments) / float64(v.Views)
		}
		perf = append(perf, models.VideoPerformancePoint{
			UploadDate:      v.UploadDate,
			Views:           v.Views,
			Likes:           v.Likes,
			Comments:        v.Comments,
			LikesToViews:    likesToViews,
			CommentsToViews: commentsToViews,
		})
	}

	// Rolling average of views in the last 10 uploads vs previous
	var rolling []models.RollingAverage
	window := 10
	for i := 0; i < len(videos); i++ {
		start := i - window + 1
		if start < 0 {
			start = 0
		}
		sum := int64(0)
		for j := start; j <= i; j++ {
			sum += videos[j].Views
		}
		avg := float64(sum) / float64(i-start+1)
		rolling = append(rolling, models.RollingAverage{
			UploadIndex:  i,
			AverageViews: avg,
		})
	}

	// 2. Upload frequency trend (per week/month)
	weekly := make(map[string]int)
	monthly := make(map[string]int)
	engageWeekly := make(map[string][]models.Video)
	engageMonthly := make(map[string][]models.Video)
	for _, v := range videos {
		wYear, w := v.UploadDate.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", wYear, w)
		monthKey := v.UploadDate.Format("2006-01")
		weekly[weekKey]++
		monthly[monthKey]++
		engageWeekly[weekKey] = append(engageWeekly[weekKey], v)
		engageMonthly[monthKey] = append(engageMonthly[monthKey], v)
	}
	var freqWeekly []models.UploadFrequency
	for k, v := range weekly {
		freqWeekly = append(freqWeekly, models.UploadFrequency{Period: k, Count: v})
	}
	var freqMonthly []models.UploadFrequency
	for k, v := range monthly {
		freqMonthly = append(freqMonthly, models.UploadFrequency{Period: k, Count: v})
	}

	// 2b. Engagement trends (weekly/monthly)
	var engageTrendsWeekly []models.EngagementTrend
	for k, vids := range engageWeekly {
		var sumViews, sumLikes, sumComments int64
		for _, v := range vids {
			sumViews += v.Views
			sumLikes += v.Likes
			sumComments += v.Comments
		}
		count := float64(len(vids))
		engageTrendsWeekly = append(engageTrendsWeekly, models.EngagementTrend{
			Period:          k,
			AverageViews:    float64(sumViews) / count,
			AverageLikes:    float64(sumLikes) / count,
			AverageComments: float64(sumComments) / count,
		})
	}
	var engageTrendsMonthly []models.EngagementTrend
	for k, vids := range engageMonthly {
		var sumViews, sumLikes, sumComments int64
		for _, v := range vids {
			sumViews += v.Views
			sumLikes += v.Likes
			sumComments += v.Comments
		}
		count := float64(len(vids))
		engageTrendsMonthly = append(engageTrendsMonthly, models.EngagementTrend{
			Period:          k,
			AverageViews:    float64(sumViews) / count,
			AverageLikes:    float64(sumLikes) / count,
			AverageComments: float64(sumComments) / count,
		})
	}

	// Get top 5 trending videos
	trendingVideos := make([]models.Video, 0)
	if len(videos) > 0 {
		// Sort by engagement score
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].EngagementScore() > videos[j].EngagementScore()
		})
		// Take top 5
		if len(videos) > 5 {
			trendingVideos = videos[:5]
		} else {
			trendingVideos = videos
		}
	}

	return &models.ChannelTrends{
		ChannelID:               channelID,
		ChannelTitle:            channel.Title,
		ChannelName:             channel.Title,
		TrendingVideos:          trendingVideos,
		PerformanceOverTime:     perf,
		RollingAverages:         rolling,
		UploadFrequencyWeekly:   freqWeekly,
		UploadFrequencyMonthly:  freqMonthly,
		EngagementTrendsWeekly:  engageTrendsWeekly,
		EngagementTrendsMonthly: engageTrendsMonthly,
		Timestamp:               time.Now(),
	}, nil
}

// YouTubeAPI handles YouTube API interactions
type YouTubeAPI struct {
	service *youtube.Service
	client  *YouTubeClient
	db      *models.Database
}

// NewYouTubeAPI creates a new YouTube API handler
func NewYouTubeAPI(apiKey string, db *models.Database) (*YouTubeAPI, error) {
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %v", err)
	}

	client := NewYouTubeClient(apiKey)

	return &YouTubeAPI{
		service: service,
		client:  client,
		db:      db,
	}, nil
}

// GetChannelAnalytics retrieves analytics for a channel
func (h *YouTubeAPI) GetChannelAnalytics(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel ID is required"})
		return
	}

	log.Printf("Fetching analytics for channel: %s", channelID)

	// Check if we have cached analytics for today
	engagement, err := h.db.GetLatestEngagement(channelID, models.EngagementTypeAnalytics)
	if err != nil {
		log.Printf("Error fetching cached analytics: %v", err)
	} else if engagement != nil {
		log.Printf("Found cached analytics from: %v", engagement.UpdateDate)
		// Check if the data is from today
		if engagement.UpdateDate.UTC().Format("2006-01-02") == time.Now().UTC().Format("2006-01-02") {
			// Return cached data without updating
			var analytics models.ChannelAnalytics
			if err := json.Unmarshal(engagement.JSONResponse, &analytics); err != nil {
				log.Printf("Failed to unmarshal cached analytics: %v", err)
			} else {
				log.Printf("Returning cached analytics without update")
				c.JSON(http.StatusOK, analytics)
				return
			}
		} else {
			log.Printf("Cached data is from a different day, fetching fresh data")
		}
	} else {
		log.Printf("No cached analytics found, fetching fresh data")
	}

	// Only fetch from YouTube API if no valid cached data exists
	log.Printf("Fetching fresh analytics from YouTube API")
	analytics, err := h.getChannelAnalytics(channelID)
	if err != nil {
		log.Printf("Error fetching analytics from YouTube API: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store the new analytics data
	jsonData, err := json.Marshal(analytics)
	if err != nil {
		log.Printf("Error marshaling analytics data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process analytics data"})
		return
	}

	engagement = &models.ChannelEngagement{
		ChannelID:      channelID,
		EngagementType: models.EngagementTypeAnalytics,
		JSONResponse:   jsonData,
	}

	log.Printf("Storing new analytics data in database")
	if err := h.db.StoreEngagement(engagement); err != nil {
		log.Printf("Failed to store analytics data: %v", err)
	} else {
		log.Printf("Successfully stored new analytics data")
	}

	c.JSON(http.StatusOK, analytics)
}

// GetChannelTrends retrieves trends for a channel
func (h *YouTubeAPI) GetChannelTrends(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel ID is required"})
		return
	}

	log.Printf("Fetching trends for channel: %s", channelID)

	// Check if we have cached trends for today
	engagement, err := h.db.GetLatestEngagement(channelID, models.EngagementTypeTrends)
	if err != nil {
		log.Printf("Error fetching cached trends: %v", err)
	} else if engagement != nil {
		log.Printf("Found cached trends from: %v", engagement.UpdateDate)
		// Check if the data is from today
		if engagement.UpdateDate.UTC().Format("2006-01-02") == time.Now().UTC().Format("2006-01-02") {
			// Return cached data without updating
			var trends models.ChannelTrends
			if err := json.Unmarshal(engagement.JSONResponse, &trends); err != nil {
				log.Printf("Failed to unmarshal cached trends: %v", err)
			} else {
				log.Printf("Returning cached trends without update")
				c.JSON(http.StatusOK, trends)
				return
			}
		} else {
			log.Printf("Cached data is from a different day, fetching fresh data")
		}
	} else {
		log.Printf("No cached trends found, fetching fresh data")
	}

	// Only fetch from YouTube API if no valid cached data exists
	log.Printf("Fetching fresh trends from YouTube API")
	trends, err := h.getChannelTrends(channelID)
	if err != nil {
		log.Printf("Error fetching trends from YouTube API: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store the new trends data
	jsonData, err := json.Marshal(trends)
	if err != nil {
		log.Printf("Error marshaling trends data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process trends data"})
		return
	}

	engagement = &models.ChannelEngagement{
		ChannelID:      channelID,
		EngagementType: models.EngagementTypeTrends,
		JSONResponse:   jsonData,
	}

	log.Printf("Storing new trends data in database")
	if err := h.db.StoreEngagement(engagement); err != nil {
		log.Printf("Failed to store trends data: %v", err)
	} else {
		log.Printf("Successfully stored new trends data")
	}

	c.JSON(http.StatusOK, trends)
}

func (y *YouTubeAPI) getChannelAnalytics(channelID string) (*models.ChannelAnalytics, error) {
	// Get channel info
	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		return nil, err
	}

	// Get channel statistics
	stats, err := y.getChannelStatistics(channelID)
	if err != nil {
		return nil, err
	}

	// Get all videos for analytics
	allVideos, err := y.getAllVideos(channelID)
	if err != nil {
		return nil, err
	}

	// Calculate analytics metrics
	var totalViews int64
	var totalLikes int64
	var totalComments int64
	var topEngagingVideos []models.Video

	// Convert videos and calculate totals
	videos := make([]models.Video, 0, len(allVideos))
	for _, v := range allVideos {
		views := int64(0)
		likes := int64(0)
		comments := int64(0)
		publishedAt := time.Time{}

		if viewCount, ok := any(v.Statistics.ViewCount).(uint64); ok {
			views = int64(viewCount)
		}
		if likeCount, ok := any(v.Statistics.LikeCount).(uint64); ok {
			likes = int64(likeCount)
		}
		if commentCount, ok := any(v.Statistics.CommentCount).(uint64); ok {
			comments = int64(commentCount)
		}
		if pub, ok := any(v.Snippet.PublishedAt).(string); ok {
			parsed, err := time.Parse(time.RFC3339, pub)
			if err == nil {
				publishedAt = parsed
			}
		} else if pub, ok := any(v.Snippet.PublishedAt).(time.Time); ok {
			publishedAt = pub
		}

		video := models.Video{
			ID:           v.Id,
			Title:        v.Snippet.Title,
			Description:  v.Snippet.Description,
			Views:        views,
			Likes:        likes,
			Comments:     comments,
			UploadDate:   publishedAt,
			PublishedAt:  publishedAt,
			Duration:     v.ContentDetails.Duration,
			Thumbnail:    v.Snippet.Thumbnails.Default.Url,
			ViewCount:    views,
			LikeCount:    likes,
			CommentCount: comments,
		}

		videos = append(videos, video)
		totalViews += views
		totalLikes += likes
		totalComments += comments
	}

	// Calculate averages and ratios
	var averageViews float64
	var likeToViewRatio float64
	var commentToViewRatio float64

	if len(videos) > 0 {
		averageViews = float64(totalViews) / float64(len(videos))
		if totalViews > 0 {
			likeToViewRatio = float64(totalLikes) / float64(totalViews)
			commentToViewRatio = float64(totalComments) / float64(totalViews)
		}
	}

	// Sort videos by engagement (views + likes + comments)
	sort.Slice(videos, func(i, j int) bool {
		iEngagement := videos[i].Views + videos[i].Likes + videos[i].Comments
		jEngagement := videos[j].Views + videos[j].Likes + videos[j].Comments
		return iEngagement > jEngagement
	})

	// Get top 5 engaging videos
	if len(videos) > 5 {
		topEngagingVideos = videos[:5]
	} else {
		topEngagingVideos = videos
	}

	// Get time range
	var startDate, endDate string
	if len(videos) > 0 {
		startDate = videos[len(videos)-1].PublishedAt.Format("2006-01-02")
		endDate = videos[0].PublishedAt.Format("2006-01-02")
	}

	return &models.ChannelAnalytics{
		ChannelID:          channelID,
		ChannelTitle:       channel.Snippet.Title,
		ChannelName:        channel.Snippet.Title,
		SubscriberCount:    int64(stats.SubscriberCount),
		ViewCount:          int64(stats.ViewCount),
		VideoCount:         int64(stats.VideoCount),
		TotalVideos:        len(videos),
		AverageViews:       averageViews,
		LikeToViewRatio:    likeToViewRatio,
		CommentToViewRatio: commentToViewRatio,
		TopEngagingVideos:  topEngagingVideos,
		TimeRange: models.TimeRange{
			StartDate: startDate,
			EndDate:   endDate,
		},
		Timestamp: time.Now(),
	}, nil
}

func (y *YouTubeAPI) getChannelTrends(channelID string) (*models.ChannelTrends, error) {
	// Get channel info
	apiChannel, err := y.getChannelInfo(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel info: %v", err)
	}

	// Convert API channel to our model
	channel := &models.Channel{
		ID:          apiChannel.Id,
		Title:       apiChannel.Snippet.Title,
		Description: apiChannel.Snippet.Description,
		Thumbnail:   apiChannel.Snippet.Thumbnails.Default.Url,
	}

	// Get all videos for trends
	apiVideos, err := y.getAllVideos(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel videos: %v", err)
	}

	// Convert API videos to our model
	videos := make([]models.Video, 0, len(apiVideos))
	for _, v := range apiVideos {
		views := int64(0)
		likes := int64(0)
		comments := int64(0)
		publishedAt := time.Time{}

		if viewCount, ok := any(v.Statistics.ViewCount).(uint64); ok {
			views = int64(viewCount)
		}
		if likeCount, ok := any(v.Statistics.LikeCount).(uint64); ok {
			likes = int64(likeCount)
		}
		if commentCount, ok := any(v.Statistics.CommentCount).(uint64); ok {
			comments = int64(commentCount)
		}
		if pub, ok := any(v.Snippet.PublishedAt).(string); ok {
			parsed, err := time.Parse(time.RFC3339, pub)
			if err == nil {
				publishedAt = parsed
			}
		} else if pub, ok := any(v.Snippet.PublishedAt).(time.Time); ok {
			publishedAt = pub
		}

		video := models.Video{
			ID:           v.Id,
			Title:        v.Snippet.Title,
			Description:  v.Snippet.Description,
			Views:        views,
			Likes:        likes,
			Comments:     comments,
			UploadDate:   publishedAt,
			PublishedAt:  publishedAt,
			Duration:     v.ContentDetails.Duration,
			Thumbnail:    v.Snippet.Thumbnails.Default.Url,
			ViewCount:    views,
			LikeCount:    likes,
			CommentCount: comments,
		}
		videos = append(videos, video)
	}

	// Sort videos by upload date
	sort.Slice(videos, func(i, j int) bool {
		return videos[i].UploadDate.After(videos[j].UploadDate)
	})

	// Calculate engagement trends
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, -1, 0)

	var weeklyViews, weeklyLikes, weeklyComments int64
	var monthlyViews, monthlyLikes, monthlyComments int64
	var weeklyCount, monthlyCount int

	for _, video := range videos {
		if video.UploadDate.After(weekAgo) {
			weeklyViews += video.Views
			weeklyLikes += video.Likes
			weeklyComments += video.Comments
			weeklyCount++
		}
		if video.UploadDate.After(monthAgo) {
			monthlyViews += video.Views
			monthlyLikes += video.Likes
			monthlyComments += video.Comments
			monthlyCount++
		}
	}

	// Calculate averages safely
	var weeklyAvgViews, weeklyAvgLikes, weeklyAvgComments float64
	var monthlyAvgViews, monthlyAvgLikes, monthlyAvgComments float64

	if weeklyCount > 0 {
		weeklyAvgViews = float64(weeklyViews) / float64(weeklyCount)
		weeklyAvgLikes = float64(weeklyLikes) / float64(weeklyCount)
		weeklyAvgComments = float64(weeklyComments) / float64(weeklyCount)
	}

	if monthlyCount > 0 {
		monthlyAvgViews = float64(monthlyViews) / float64(monthlyCount)
		monthlyAvgLikes = float64(monthlyLikes) / float64(monthlyCount)
		monthlyAvgComments = float64(monthlyComments) / float64(monthlyCount)
	}

	engagementTrendsWeekly := []models.EngagementTrend{
		{
			Period:          "week",
			AverageViews:    weeklyAvgViews,
			AverageLikes:    weeklyAvgLikes,
			AverageComments: weeklyAvgComments,
		},
	}

	engagementTrendsMonthly := []models.EngagementTrend{
		{
			Period:          "month",
			AverageViews:    monthlyAvgViews,
			AverageLikes:    monthlyAvgLikes,
			AverageComments: monthlyAvgComments,
		},
	}

	// Calculate performance over time safely
	performanceOverTime := make([]models.VideoPerformancePoint, 0)
	for _, video := range videos {
		likesToViews := 0.0
		commentsToViews := 0.0
		if video.Views > 0 {
			likesToViews = float64(video.Likes) / float64(video.Views)
			commentsToViews = float64(video.Comments) / float64(video.Views)
		}
		point := models.VideoPerformancePoint{
			UploadDate:      video.UploadDate,
			Views:           video.Views,
			Likes:           video.Likes,
			Comments:        video.Comments,
			LikesToViews:    likesToViews,
			CommentsToViews: commentsToViews,
		}
		performanceOverTime = append(performanceOverTime, point)
	}

	// Calculate rolling averages safely
	rollingAverages := make([]models.RollingAverage, 0)
	windowSize := 7
	for i := 0; i < len(videos); i++ {
		end := i + windowSize
		if end > len(videos) {
			end = len(videos)
		}
		window := videos[i:end]

		var totalViews int64
		for _, v := range window {
			totalViews += v.Views
		}

		avgViews := 0.0
		if len(window) > 0 {
			avgViews = float64(totalViews) / float64(len(window))
		}

		avg := models.RollingAverage{
			UploadIndex:  i,
			AverageViews: avgViews,
		}
		rollingAverages = append(rollingAverages, avg)
	}

	// Calculate upload frequency
	now = time.Now()
	weekAgo = now.AddDate(0, 0, -7)
	monthAgo = now.AddDate(0, -1, 0)

	var weeklyUploads, monthlyUploads int
	for _, video := range videos {
		if video.UploadDate.After(weekAgo) {
			weeklyUploads++
		}
		if video.UploadDate.After(monthAgo) {
			monthlyUploads++
		}
	}

	uploadFreqWeekly := []models.UploadFrequency{
		{Period: "week", Count: weeklyUploads},
	}
	uploadFreqMonthly := []models.UploadFrequency{
		{Period: "month", Count: monthlyUploads},
	}

	// Get top 5 trending videos
	trendingVideos := make([]models.Video, 0)
	if len(videos) > 0 {
		// Sort by engagement score
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].EngagementScore() > videos[j].EngagementScore()
		})
		// Take top 5
		if len(videos) > 5 {
			trendingVideos = videos[:5]
		} else {
			trendingVideos = videos
		}
	}

	return &models.ChannelTrends{
		ChannelID:               channelID,
		ChannelTitle:            channel.Title,
		ChannelName:             channel.Title,
		TrendingVideos:          trendingVideos,
		PerformanceOverTime:     performanceOverTime,
		RollingAverages:         rollingAverages,
		UploadFrequencyWeekly:   uploadFreqWeekly,
		UploadFrequencyMonthly:  uploadFreqMonthly,
		EngagementTrendsWeekly:  engagementTrendsWeekly,
		EngagementTrendsMonthly: engagementTrendsMonthly,
		Timestamp:               time.Now(),
	}, nil
}

func (y *YouTubeAPI) getChannelInfo(channelID string) (*youtube.Channel, error) {
	// Request both snippet, statistics, and contentDetails parts
	call := y.service.Channels.List([]string{"snippet", "statistics", "contentDetails"}).Id(channelID)
	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching channel info: %v", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("channel not found")
	}

	channel := response.Items[0]

	// Log channel details for debugging
	fmt.Printf("Channel found: %s\n", channel.Snippet.Title)
	fmt.Printf("ContentDetails: %+v\n", channel.ContentDetails)
	if channel.ContentDetails != nil {
		fmt.Printf("RelatedPlaylists: %+v\n", channel.ContentDetails.RelatedPlaylists)
	}

	return channel, nil
}

func (y *YouTubeAPI) getChannelStatistics(channelID string) (*youtube.ChannelStatistics, error) {
	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		return nil, err
	}
	return channel.Statistics, nil
}

func (y *YouTubeAPI) getAllVideos(channelID string) ([]*youtube.Video, error) {
	var allVideos []*youtube.Video
	var nextPageToken string

	// Get channel's uploads playlist ID
	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		return nil, fmt.Errorf("error getting channel info: %v", err)
	}

	if channel == nil || channel.ContentDetails == nil || channel.ContentDetails.RelatedPlaylists == nil {
		return nil, fmt.Errorf("channel or related playlists not found")
	}

	playlistID := channel.ContentDetails.RelatedPlaylists.Uploads
	if playlistID == "" {
		return nil, fmt.Errorf("uploads playlist ID not found")
	}

	for {
		// Get videos from the uploads playlist
		call := y.service.PlaylistItems.List([]string{"snippet"}).
			PlaylistId(playlistID).
			MaxResults(50)
		if nextPageToken != "" {
			call = call.PageToken(nextPageToken)
		}

		response, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("error fetching videos: %v", err)
		}

		if response == nil || len(response.Items) == 0 {
			break
		}

		// Get video IDs
		var videoIDs []string
		for _, item := range response.Items {
			if item != nil && item.Snippet != nil && item.Snippet.ResourceId != nil {
				videoIDs = append(videoIDs, item.Snippet.ResourceId.VideoId)
			}
		}

		// Get video details
		if len(videoIDs) > 0 {
			videoCall := y.service.Videos.List([]string{"snippet", "statistics", "contentDetails"}).
				Id(videoIDs...)
			videoResponse, err := videoCall.Do()
			if err != nil {
				return nil, fmt.Errorf("error fetching video details: %v", err)
			}

			if videoResponse != nil && videoResponse.Items != nil {
				allVideos = append(allVideos, videoResponse.Items...)
			}
		}

		// Check if there are more pages
		nextPageToken = response.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return allVideos, nil
}

func (y *YouTubeAPI) GetChannelByURL(c *gin.Context) {
	channelURL := c.Query("url")
	if channelURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "YouTube URL is required"})
		return
	}

	// Extract channel ID from URL
	channelID, err := y.client.ExtractChannelIDFromURL(channelURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid YouTube URL: %v", err)})
		return
	}

	// Get channel info with a single API call
	call := y.service.Channels.List([]string{"snippet", "statistics", "contentDetails"}).
		Id(channelID).
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching channel info: %v", err)})
		return
	}

	if len(response.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	item := response.Items[0]
	channel := &models.Channel{
		ID:          item.Id,
		Title:       item.Snippet.Title,
		Description: item.Snippet.Description,
		Subscribers: int64(item.Statistics.SubscriberCount),
		ViewCount:   int64(item.Statistics.ViewCount),
		VideoCount:  int64(item.Statistics.VideoCount),
		Thumbnail:   item.Snippet.Thumbnails.Default.Url,
	}

	c.JSON(http.StatusOK, channel)
}

func (y *YouTubeAPI) GetChannelByID(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel ID is required"})
		return
	}

	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching channel info: %v", err)})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (y *YouTubeAPI) GetChannelByTitle(c *gin.Context) {
	title := c.Param("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel title is required"})
		return
	}

	// Search for the channel
	call := y.service.Search.List([]string{"snippet"}).
		Q(title).
		Type("channel").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error searching for channel: %v", err)})
		return
	}

	if len(response.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	// Get channel details
	channelID := response.Items[0].Id.ChannelId
	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching channel info: %v", err)})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (y *YouTubeAPI) GetChannelVideos(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Channel ID is required"})
		return
	}

	// Get channel info first to check if it exists
	channel, err := y.getChannelInfo(channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Error fetching channel info: %v", err),
			"details": "Failed to retrieve channel information from YouTube API",
		})
		return
	}

	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Channel not found",
			"details": "The requested channel ID does not exist",
		})
		return
	}

	if channel.ContentDetails == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Channel content details not found",
			"details": "The channel exists but content details are not available",
		})
		return
	}

	if channel.ContentDetails.RelatedPlaylists == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Channel playlists not found",
			"details": "The channel exists but playlists information is not available",
		})
		return
	}

	if channel.ContentDetails.RelatedPlaylists.Uploads == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Uploads playlist not found",
			"details": "The channel exists but uploads playlist ID is not available",
		})
		return
	}

	// Parse filter parameters
	filter := models.VideoFilter{
		MaxVideos: 50, // Default value
	}

	// Get sort option
	sortBy := c.Query("sortBy")
	switch sortBy {
	case "views":
		filter.SortBy = models.SortByViews
	case "likes":
		filter.SortBy = models.SortByLikes
	case "recency":
		filter.SortBy = models.SortByRecency
	default:
		filter.SortBy = models.SortByRecency
	}

	// Get max videos
	if maxVideos := c.Query("maxVideos"); maxVideos != "" {
		if n, err := strconv.Atoi(maxVideos); err == nil && n > 0 {
			filter.MaxVideos = n
		}
	}

	// Get minimum views
	if minViews := c.Query("minViews"); minViews != "" {
		if n, err := strconv.ParseInt(minViews, 10, 64); err == nil {
			filter.MinViews = n
		}
	}

	// Get minimum likes
	if minLikes := c.Query("minLikes"); minLikes != "" {
		if n, err := strconv.ParseInt(minLikes, 10, 64); err == nil {
			filter.MinLikes = n
		}
	}

	// Get all videos
	allVideos, err := y.getAllVideos(channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   fmt.Sprintf("Error fetching videos: %v", err),
			"details": "Failed to retrieve videos from YouTube API",
		})
		return
	}

	if len(allVideos) == 0 {
		c.JSON(http.StatusOK, []models.Video{})
		return
	}

	// Convert to Video model and apply filters
	videos := make([]models.Video, 0, len(allVideos))
	for _, v := range allVideos {
		if v == nil || v.Snippet == nil || v.Statistics == nil || v.ContentDetails == nil {
			continue
		}

		// For Go client: v.Statistics.ViewCount is uint64, v.Snippet.PublishedAt is string
		views := int64(0)
		likes := int64(0)
		comments := int64(0)
		publishedAt := time.Time{}

		if viewCount, ok := any(v.Statistics.ViewCount).(uint64); ok {
			views = int64(viewCount)
		}
		if likeCount, ok := any(v.Statistics.LikeCount).(uint64); ok {
			likes = int64(likeCount)
		}
		if commentCount, ok := any(v.Statistics.CommentCount).(uint64); ok {
			comments = int64(commentCount)
		}
		if pub, ok := any(v.Snippet.PublishedAt).(string); ok {
			parsed, err := time.Parse(time.RFC3339, pub)
			if err == nil {
				publishedAt = parsed
			}
		} else if pub, ok := any(v.Snippet.PublishedAt).(time.Time); ok {
			publishedAt = pub
		}

		// Apply filters
		if views < filter.MinViews || likes < filter.MinLikes {
			continue
		}

		videos = append(videos, models.Video{
			ID:           v.Id,
			Title:        v.Snippet.Title,
			Description:  v.Snippet.Description,
			Views:        views,
			Likes:        likes,
			Comments:     comments,
			UploadDate:   publishedAt,
			PublishedAt:  publishedAt,
			Duration:     v.ContentDetails.Duration,
			Thumbnail:    v.Snippet.Thumbnails.Default.Url,
			ViewCount:    views,
			LikeCount:    likes,
			CommentCount: comments,
		})
	}

	// Sort videos based on filter
	switch filter.SortBy {
	case models.SortByViews:
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].Views > videos[j].Views
		})
	case models.SortByLikes:
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].Likes > videos[j].Likes
		})
	case models.SortByRecency:
		sort.Slice(videos, func(i, j int) bool {
			return videos[i].UploadDate.After(videos[j].UploadDate)
		})
	}

	// Limit to requested number of videos
	if len(videos) > filter.MaxVideos {
		videos = videos[:filter.MaxVideos]
	}

	c.JSON(http.StatusOK, videos)
}
