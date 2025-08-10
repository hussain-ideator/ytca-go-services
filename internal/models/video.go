package models

import "time"

// Video represents a YouTube video
type Video struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Views        int64     `json:"views"`
	Likes        int64     `json:"likes"`
	Comments     int64     `json:"comments"`
	UploadDate   time.Time `json:"uploadDate"`
	PublishedAt  time.Time `json:"publishedAt"`
	Duration     string    `json:"duration"`
	Thumbnail    string    `json:"thumbnailUrl"`
	ViewCount    int64     `json:"viewCount"`
	LikeCount    int64     `json:"likeCount"`
	CommentCount int64     `json:"commentCount"`
}

// VideoListResponse represents the response from YouTube API for video list
type VideoListResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string    `json:"title"`
			Description string    `json:"description"`
			PublishedAt time.Time `json:"publishedAt"`
			Thumbnails  struct {
				Default struct {
					URL string `json:"url"`
				} `json:"default"`
			} `json:"thumbnails"`
		} `json:"snippet"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Statistics struct {
			ViewCount    string `json:"viewCount"`
			LikeCount    string `json:"likeCount"`
			CommentCount string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
}

// VideoSortOption represents the available sorting options
type VideoSortOption string

const (
	SortByViews   VideoSortOption = "views"
	SortByLikes   VideoSortOption = "likes"
	SortByRecency VideoSortOption = "recency"
)

// VideoFilter represents the filter options for videos
type VideoFilter struct {
	SortBy    VideoSortOption `json:"sortBy"`
	MaxVideos int             `json:"maxVideos"`
	MinViews  int64           `json:"minViews"`
	MinLikes  int64           `json:"minLikes"`
}
