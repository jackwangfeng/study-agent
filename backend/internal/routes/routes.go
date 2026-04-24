package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackwangfeng/study-agent/backend/internal/auth"
	"github.com/jackwangfeng/study-agent/backend/internal/config"
	"github.com/jackwangfeng/study-agent/backend/internal/handlers"
	"github.com/jackwangfeng/study-agent/backend/internal/middleware"
	"github.com/jackwangfeng/study-agent/backend/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func SetupUserRoutes(v1 *gin.RouterGroup, db *gorm.DB, logger *zap.Logger) {
	userService := services.NewUserService(db, logger)
	userHandler := handlers.NewUserHandler(userService, logger)

	users := v1.Group("/users")
	{
		users.POST("/profile", userHandler.CreateProfile)
		users.GET("/profile/:id", userHandler.GetProfile)
		users.GET("/profile/openid/:openid", userHandler.GetProfileByOpenID)
		users.PUT("/profile/:id", userHandler.UpdateProfile)
		users.DELETE("/profile/:id", userHandler.DeleteProfile)
	}
}

func SetupChemRoutes(v1 *gin.RouterGroup, db *gorm.DB, logger *zap.Logger, cfg *config.Config) {
	svc := services.NewChemService(db, logger, cfg.LLMAPIKey, cfg.LLMAPIURL, cfg.LLMModel, cfg.Debug)
	h := handlers.NewChemHandler(svc, logger)

	chem := v1.Group("/chem")
	{
		chem.POST("/solve", h.Solve)
		chem.POST("/mistake", h.LogMistake)
		chem.GET("/mistakes", h.ListMistakes)
		chem.GET("/review/due", h.DueReview)
	}
}

func SetupFeedbackRoutes(v1 *gin.RouterGroup, db *gorm.DB, logger *zap.Logger) {
	fbService := services.NewFeedbackService(db, logger)
	fbHandler := handlers.NewFeedbackHandler(fbService, logger)
	v1.POST("/feedback", fbHandler.Submit)
}

func SetupAuthRoutes(v1 *gin.RouterGroup, db *gorm.DB, logger *zap.Logger, googleClientID, googleIOSClientID string, tokens *auth.TokenIssuer) {
	authService := services.NewAuthService(db, logger, googleClientID, googleIOSClientID, tokens)
	authHandler := handlers.NewAuthHandler(authService, logger)

	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/sms/send", authHandler.SendSMS)
		authGroup.POST("/sms/login", authHandler.PhoneLogin)
		authGroup.POST("/google", authHandler.GoogleLogin)

		protected := authGroup.Group("")
		protected.Use(middleware.AuthRequired(tokens))
		protected.GET("/me", authHandler.GetCurrentUser)
		protected.POST("/logout", authHandler.Logout)
	}
}
