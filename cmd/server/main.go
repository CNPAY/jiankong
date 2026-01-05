package main

import (
	"domain-monitor/internal/api"
	"domain-monitor/internal/config"
	"domain-monitor/internal/database"
	"domain-monitor/internal/models"
	"domain-monitor/internal/scheduler"
	"domain-monitor/internal/services"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// loadSettingsFromDB loads settings from database and overrides config
func loadSettingsFromDB(cfg *config.Config) {
	db := database.GetDB()
	if db == nil {
		return
	}

	var settings []models.Setting
	if err := db.Find(&settings).Error; err != nil {
		log.Printf("Warning: Failed to load settings from database: %v", err)
		return
	}

	// Convert settings array to map
	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	// Override monitor settings
	if val, ok := settingsMap["monitor.check_interval"]; ok && val != "" {
		cfg.Monitor.CheckInterval = val
	}
	if val, ok := settingsMap["monitor.alert_days"]; ok && val != "" {
		// Parse comma-separated days
		days := []int{}
		for _, d := range strings.Split(val, ",") {
			if day, err := strconv.Atoi(strings.TrimSpace(d)); err == nil {
				days = append(days, day)
			}
		}
		if len(days) > 0 {
			cfg.Monitor.AlertDays = days
		}
	}

	// Override email settings
	if val, ok := settingsMap["email.enabled"]; ok {
		cfg.Notifications.Email.Enabled = val == "true"
	}
	if val, ok := settingsMap["email.smtp_host"]; ok {
		cfg.Notifications.Email.SMTPHost = val
	}
	if val, ok := settingsMap["email.smtp_port"]; ok {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Notifications.Email.SMTPPort = port
		}
	}
	if val, ok := settingsMap["email.from"]; ok {
		cfg.Notifications.Email.From = val
	}
	if val, ok := settingsMap["email.password"]; ok {
		cfg.Notifications.Email.Password = val
	}
	if val, ok := settingsMap["email.to"]; ok && val != "" {
		cfg.Notifications.Email.To = strings.Split(val, ",")
	}

	// Override webhook settings
	if val, ok := settingsMap["webhook.enabled"]; ok {
		cfg.Notifications.Webhook.Enabled = val == "true"
	}
	if val, ok := settingsMap["webhook.url"]; ok {
		cfg.Notifications.Webhook.URL = val
	}

	// Override telegram settings
	if val, ok := settingsMap["telegram.enabled"]; ok {
		cfg.Notifications.Telegram.Enabled = val == "true"
	}
	if val, ok := settingsMap["telegram.bot_token"]; ok {
		cfg.Notifications.Telegram.BotToken = val
	}
	if val, ok := settingsMap["telegram.chat_id"]; ok {
		cfg.Notifications.Telegram.ChatID = val
	}

	// Override dingding settings
	if val, ok := settingsMap["dingding.enabled"]; ok {
		cfg.Notifications.DingDing.Enabled = val == "true"
	}
	if val, ok := settingsMap["dingding.webhook"]; ok {
		cfg.Notifications.DingDing.Webhook = val
	}
	if val, ok := settingsMap["dingding.secret"]; ok {
		cfg.Notifications.DingDing.Secret = val
	}

	log.Println("Settings loaded from database and applied to configuration")
}

// initDefaultAdmin initializes the default admin account
func initDefaultAdmin(authService *services.AuthService) {
	db := database.GetDB()

	// Check if admin user already exists
	var existingUser models.User
	if err := db.Where("username = ?", "admin").First(&existingUser).Error; err == nil {
		log.Println("Admin account already exists")
		return
	}

	// Create default admin account (username: admin, password: admin123)
	hashedPassword, err := authService.HashPassword("admin123")
	if err != nil {
		log.Printf("Failed to hash default admin password: %v", err)
		return
	}

	admin := models.User{
		Username:  "admin",
		Password:  hashedPassword,
		Email:     "admin@example.com",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(&admin).Error; err != nil {
		log.Printf("Failed to create default admin account: %v", err)
		return
	}

	log.Println("Default admin account created (username: admin, password: admin123)")
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	if err := database.InitDB(&cfg.Database); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized successfully")

	// Load settings from database and override config
	loadSettingsFromDB(cfg)

	// Parse WHOIS timeout
	timeout, err := time.ParseDuration(cfg.Whois.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}

	// Initialize services
	whoisService := services.NewWhoisService(cfg.Whois.APIURL, timeout)
	notifyService := services.NewNotifyService(&cfg.Notifications)
	monitorService := services.NewMonitorService(whoisService, notifyService, cfg.Monitor.AlertDays)
	authService := services.NewAuthService()

	// Initialize default admin account
	initDefaultAdmin(authService)

	// Initialize scheduler
	sched := scheduler.NewScheduler(monitorService)
	if err := sched.Start(cfg.Monitor.CheckInterval); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// Setup Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Setup API routes
	handler := api.NewHandler(monitorService, whoisService, authService)
	api.SetupRoutes(r, handler)

	// Serve static files
	r.Static("/static", "./web/dist")

	// Serve frontend
	r.GET("/", func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	// Start server
	addr := ":" + cfg.Server.Port
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
