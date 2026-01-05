package api

import (
	"domain-monitor/internal/database"
	"domain-monitor/internal/models"
	"domain-monitor/internal/services"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler holds service dependencies
type Handler struct {
	monitorService *services.MonitorService
	whoisService   *services.WhoisService
	authService    *services.AuthService
}

// NewHandler creates a new API handler
func NewHandler(monitorService *services.MonitorService, whoisService *services.WhoisService, authService *services.AuthService) *Handler {
	return &Handler{
		monitorService: monitorService,
		whoisService:   whoisService,
		authService:    authService,
	}
}

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, handler *Handler) {
	api := r.Group("/api/v1")
	{
		// Authentication (no auth required)
		api.POST("/auth/login", handler.Login)
		api.POST("/auth/validate", handler.ValidateToken)
		api.POST("/auth/change-password", handler.ChangePassword)

		// Domain management
		api.GET("/domains", handler.ListDomains)
		api.POST("/domains", handler.CreateDomain)
		api.GET("/domains/:id", handler.GetDomain)
		api.PUT("/domains/:id", handler.UpdateDomain)
		api.DELETE("/domains/:id", handler.DeleteDomain)
		api.POST("/domains/import", handler.ImportDomains)
		api.GET("/domains/:id/refresh", handler.RefreshDomain)

		// Dashboard statistics
		api.GET("/dashboard/stats", handler.GetStats)
		api.GET("/dashboard/expiring", handler.GetExpiring)

		// Notifications
		api.GET("/notifications", handler.ListNotifications)

		// System settings
		api.GET("/settings", handler.GetSettings)
		api.PUT("/settings", handler.UpdateSettings)

		// Testing
		api.POST("/test/notification/:id", handler.TestNotification)
	}
}

// ListDomains retrieves all domains
func (h *Handler) ListDomains(c *gin.Context) {
	db := database.GetDB()

	var domains []models.Domain
	if err := db.Order("expiry_date asc").Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// CreateDomain adds a new domain
func (h *Handler) CreateDomain(c *gin.Context) {
	var domain models.Domain
	if err := c.ShouldBindJSON(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()

	// Set initial values
	domain.CreatedAt = time.Now()
	domain.UpdatedAt = time.Now()
	domain.IsActive = true

	if err := db.Create(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Immediately check the domain
	go h.monitorService.CheckDomain(&domain)

	c.JSON(http.StatusCreated, domain)
}

// GetDomain retrieves a single domain
func (h *Handler) GetDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	db := database.GetDB()

	var domain models.Domain
	if err := db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	c.JSON(http.StatusOK, domain)
}

// UpdateDomain updates a domain
func (h *Handler) UpdateDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	db := database.GetDB()

	var domain models.Domain
	if err := db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	if err := c.ShouldBindJSON(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domain.UpdatedAt = time.Now()

	if err := db.Save(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain)
}

// DeleteDomain removes a domain
func (h *Handler) DeleteDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	db := database.GetDB()

	if err := db.Delete(&models.Domain{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted successfully"})
}

// ImportDomains imports multiple domains
func (h *Handler) ImportDomains(c *gin.Context) {
	var request struct {
		Domains []string `json:"domains"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()
	imported := 0

	for _, domainName := range request.Domains {
		domain := models.Domain{
			Name:      domainName,
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := db.Create(&domain).Error; err != nil {
			continue
		}

		imported++
		go h.monitorService.CheckDomain(&domain)
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    len(request.Domains),
		"imported": imported,
	})
}

// RefreshDomain manually refreshes a domain's WHOIS data
func (h *Handler) RefreshDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	db := database.GetDB()

	var domain models.Domain
	if err := db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	if err := h.monitorService.CheckDomain(&domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domain)
}

// GetStats retrieves dashboard statistics
func (h *Handler) GetStats(c *gin.Context) {
	db := database.GetDB()

	var total int64
	db.Model(&models.Domain{}).Count(&total)

	var active int64
	db.Model(&models.Domain{}).Where("is_active = ?", true).Count(&active)

	var expiringSoon int64
	db.Model(&models.Domain{}).Where("days_remaining <= ? AND days_remaining > 0", 30).Count(&expiringSoon)

	var expired int64
	db.Model(&models.Domain{}).Where("days_remaining <= ?", 0).Count(&expired)

	c.JSON(http.StatusOK, gin.H{
		"total":          total,
		"active":         active,
		"expiring_soon":  expiringSoon,
		"expired":        expired,
	})
}

// GetExpiring retrieves domains expiring soon
func (h *Handler) GetExpiring(c *gin.Context) {
	db := database.GetDB()

	var domains []models.Domain
	if err := db.Where("days_remaining <= ? AND days_remaining > 0", 30).
		Order("days_remaining asc").
		Limit(10).
		Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, domains)
}

// ListNotifications retrieves notification history
func (h *Handler) ListNotifications(c *gin.Context) {
	db := database.GetDB()

	var notifications []models.Notification
	if err := db.Order("sent_at desc").Limit(100).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// GetSettings retrieves system settings
func (h *Handler) GetSettings(c *gin.Context) {
	db := database.GetDB()

	var settings []models.Setting
	if err := db.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates system settings
func (h *Handler) UpdateSettings(c *gin.Context) {
	var settings map[string]string
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.GetDB()

	for key, value := range settings {
		setting := models.Setting{
			Key:   key,
			Value: value,
		}
		db.Save(&setting)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// TestNotification manually triggers a notification for testing
func (h *Handler) TestNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	db := database.GetDB()

	var domain models.Domain
	if err := db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Trigger notification directly
	if err := h.monitorService.TriggerNotification(&domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test notification sent successfully"})
}

// Login handles user login
func (h *Handler) Login(c *gin.Context) {
	var loginReq struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	db := database.GetDB()

	// Find user by username
	var user models.User
	if err := db.Where("username = ?", loginReq.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// Check if account is active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账户已被禁用"})
		return
	}

	// Verify password
	if !h.authService.CheckPassword(user.Password, loginReq.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// Generate JWT token
	token, err := h.authService.GenerateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// ValidateToken validates JWT token
func (h *Handler) ValidateToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token 不能为空"})
		return
	}

	claims, err := h.authService.ValidateToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的 token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":       claims.UserID,
			"username": claims.Username,
		},
	})
}

// ChangePassword handles password change
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所有字段都不能为空"})
		return
	}

	// Validate new password length
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度至少为 6 位"})
		return
	}

	db := database.GetDB()

	// Find user by username
	var user models.User
	if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或旧密码错误"})
		return
	}

	// Verify old password
	if !h.authService.CheckPassword(user.Password, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或旧密码错误"})
		return
	}

	// Hash new password
	hashedPassword, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	// Update password
	user.Password = hashedPassword
	user.UpdatedAt = time.Now()

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功，请使用新密码登录"})
}
