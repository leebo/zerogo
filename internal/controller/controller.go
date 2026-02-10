package controller

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/unicornultrafoundation/zerogo/internal/config"
	"gorm.io/gorm"
)

// Controller is the centralized management server.
type Controller struct {
	db        *gorm.DB
	router    *gin.Engine
	ws        *WSHandler
	jwtSecret string
	config    *config.ControllerConfig
	log       *slog.Logger
}

// New creates a new Controller instance.
func New(cfg *config.ControllerConfig, log *slog.Logger) (*Controller, error) {
	// Initialize database
	db, err := InitDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	ctrl := &Controller{
		db:        db,
		jwtSecret: cfg.JWTSecret,
		config:    cfg,
		log:       log,
	}

	// Create default admin user if none exists
	if err := ctrl.ensureAdminUser(cfg.Admin.Username, cfg.Admin.Password); err != nil {
		return nil, fmt.Errorf("create admin user: %w", err)
	}

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	ctrl.router = router
	ctrl.ws = NewWSHandler(ctrl, log)
	ctrl.SetupRoutes(router)

	return ctrl, nil
}

// Run starts the controller HTTP server.
func (ctrl *Controller) Run() error {
	ctrl.log.Info("controller starting", "listen", ctrl.config.Listen)
	return ctrl.router.Run(ctrl.config.Listen)
}

func (ctrl *Controller) ensureAdminUser(username, password string) error {
	var count int64
	ctrl.db.Model(&User{}).Count(&count)
	if count > 0 {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	user := User{
		Username: username,
		Password: hash,
		Role:     "admin",
	}
	return ctrl.db.Create(&user).Error
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Node-Address, X-Public-Key")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
