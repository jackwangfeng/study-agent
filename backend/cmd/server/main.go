package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jackwangfeng/study-agent/backend/internal/auth"
	"github.com/jackwangfeng/study-agent/backend/internal/config"
	"github.com/jackwangfeng/study-agent/backend/internal/database"
	"github.com/jackwangfeng/study-agent/backend/internal/middleware"
	"github.com/jackwangfeng/study-agent/backend/internal/models"
	"github.com/jackwangfeng/study-agent/backend/internal/routes"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config", "config file path")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Auto migrate models
	if err := database.Migrate(db,
		&models.UserProfile{},
		&models.UserSettings{},
		&models.SMSCode{},
		&models.UserAccount{},
		&models.Feedback{},
		&models.Mistake{},
		&models.Variant{},
	); err != nil {
		logger.Fatal("Failed to migrate database", zap.Error(err))
	}

	// Set Gin mode
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	r := gin.Default()

	// Apply middleware
	r.Use(middleware.CORS())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})

	// API v1 routes
	v1 := r.Group("/v1")
	{
		tokens := auth.NewTokenIssuer(cfg.SecretKey, cfg.JWTExpireDays)
		routes.SetupAuthRoutes(v1, db, logger, cfg.GoogleClientID, cfg.GoogleIOSClientID, tokens)
		routes.SetupUserRoutes(v1, db, logger)
		routes.SetupChemRoutes(v1, db, logger, cfg)
		routes.SetupFeedbackRoutes(v1, db, logger)
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Starting server", zap.String("address", addr))

	if err := r.Run(addr); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
