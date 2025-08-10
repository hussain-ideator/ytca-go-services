package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yt-insights/internal/config"
	"github.com/yt-insights/internal/models"
)

// Server represents the API server
type Server struct {
	router *gin.Engine
	client *YouTubeClient
}

// NewServer creates a new API server
func NewServer(cfg *config.Config) *Server {
	router := gin.Default()

	// Custom CORS middleware
	router.Use(func(c *gin.Context) {
		// Log the incoming request headers for debugging
		log.Printf("Incoming request headers: %v", c.Request.Header)
		log.Printf("Request origin: %s", c.Request.Header.Get("Origin"))

		// Get the origin from the request
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := map[string]bool{
			"http://localhost:3000": true,
			"http://localhost:3001": true,
		}

		// Always set CORS headers for preflight requests
		if c.Request.Method == "OPTIONS" {
			if allowedOrigins[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, Cache-Control, cache-control, Pragma, pragma, If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
				c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Cache-Control, cache-control, ETag, Last-Modified")
				c.Header("Access-Control-Max-Age", "43200")

				// Log the response headers
				log.Printf("Setting preflight response headers: %v", c.Writer.Header())

				c.AbortWithStatus(204)
				return
			}
		}

		// For non-preflight requests, set CORS headers if origin is allowed
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With, Cache-Control, cache-control, Pragma, pragma, If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Cache-Control, cache-control, ETag, Last-Modified")
		}

		c.Next()
	})

	client := NewYouTubeClient(cfg.YouTubeAPIKey)

	server := &Server{
		router: router,
		client: client,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all the routes for the server
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Channel endpoints
	s.router.GET("/channel/id/:id", s.getChannelByID)
	s.router.GET("/channel/title/:title", s.getChannelByTitle)
	s.router.GET("/channel/url", s.getChannelByURL)

	// Video endpoints
	s.router.GET("/channel/:id/videos", s.getChannelVideos)

	// Analytics endpoints
	s.router.GET("/channel/:id/analytics", s.getChannelAnalytics)
	s.router.GET("/channel/:id/trends", s.getChannelTrends)
}

// getChannelByID handles requests to get channel by ID
func (s *Server) getChannelByID(c *gin.Context) {
	channelID := c.Param("id")
	channel, err := s.client.GetChannelByID(channelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, channel)
}

// getChannelByTitle handles requests to get channel by title
func (s *Server) getChannelByTitle(c *gin.Context) {
	title := c.Param("title")
	channel, err := s.client.SearchChannelByTitle(title)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, channel)
}

// getChannelByURL handles requests to get channel by URL
func (s *Server) getChannelByURL(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "url query parameter is required",
		})
		return
	}

	channelID, err := s.client.ExtractChannelIDFromURL(url)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	channel, err := s.client.GetChannelByID(channelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, channel)
}

// getChannelVideos handles requests to get channel videos
func (s *Server) getChannelVideos(c *gin.Context) {
	channelID := c.Param("id")

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

	videos, err := s.client.GetChannelVideos(channelID, filter)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, videos)
}

// getChannelAnalytics handles requests to get channel analytics
func (s *Server) getChannelAnalytics(c *gin.Context) {
	channelID := c.Param("id")

	analytics, err := s.client.GetChannelAnalytics(channelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// getChannelTrends handles requests to get channel trends
func (s *Server) getChannelTrends(c *gin.Context) {
	channelID := c.Param("id")
	trends, err := s.client.GetChannelTrends(channelID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if trends == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No videos found for channel"})
		return
	}
	c.JSON(http.StatusOK, trends)
}

// Start starts the server on the specified port
func (s *Server) Start(port string) error {
	return s.router.Run(":" + port)
}
