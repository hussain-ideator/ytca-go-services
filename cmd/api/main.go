package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/yt-insights/internal/api"
	"github.com/yt-insights/internal/config"
	"github.com/yt-insights/internal/models"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := models.NewDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize YouTube API
	youtubeAPI, err := api.NewYouTubeAPI(cfg.YouTubeAPIKey, db)
	if err != nil {
		log.Fatalf("Failed to initialize YouTube API: %v", err)
	}

	// Initialize router
	router := gin.Default()

	// Configure CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3002", "https://ytca-frontend.vercel.app/"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "Pragma"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Register routes
	router.GET("/channel/url", youtubeAPI.GetChannelByURL)
	router.GET("/channel/:id", youtubeAPI.GetChannelByID)
	router.GET("/channel/:id/videos", youtubeAPI.GetChannelVideos)
	router.GET("/channel/:id/analytics", youtubeAPI.GetChannelAnalytics)
	router.GET("/channel/:id/trends", youtubeAPI.GetChannelTrends)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
