package models

// Channel represents a YouTube channel
type Channel struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Subscribers int64  `json:"subscriberCount"`
	ViewCount   int64  `json:"viewCount"`
	VideoCount  int64  `json:"videoCount"`
	Thumbnail   string `json:"thumbnailUrl"`
}

// ChannelResponse represents the response from YouTube API
type ChannelResponse struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumbnails  struct {
				Default struct {
					URL string `json:"url"`
				} `json:"default"`
			} `json:"thumbnails"`
		} `json:"snippet"`
		Statistics struct {
			SubscriberCount string `json:"subscriberCount"`
			ViewCount       string `json:"viewCount"`
			VideoCount      string `json:"videoCount"`
		} `json:"statistics"`
	} `json:"items"`
}
