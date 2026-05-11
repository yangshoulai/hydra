package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/repository"
	"github.com/yangshoulai/hydra/internal/service/config"
	notificationService "github.com/yangshoulai/hydra/internal/service/notification"
)

// SettingsHandler 系统设置处理器
type SettingsHandler struct {
	logger            *slog.Logger
	systemSettingRepo *repository.SystemSettingRepository
	settingService    *config.SettingService
	notificationSvc   *notificationService.Service
}

// NewSettingsHandler 创建系统设置处理器
func NewSettingsHandler(
	logger *slog.Logger,
	systemSettingRepo *repository.SystemSettingRepository,
	settingService *config.SettingService,
	notificationSvc *notificationService.Service,
) *SettingsHandler {
	return &SettingsHandler{
		logger:            logger,
		systemSettingRepo: systemSettingRepo,
		settingService:    settingService,
		notificationSvc:   notificationSvc,
	}
}

// GetAllSettings 获取所有系统设置
// GET /admin/api/settings
func (h *SettingsHandler) GetAllSettings(c *gin.Context) {
	settings, err := h.systemSettingRepo.GetAll(c.Request.Context())
	if err != nil {
		h.logger.Error("获取系统配置异常", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get settings",
		})
		return
	}

	// 转换为 map 格式返回
	settingsMap := make(map[string]string)
	for _, setting := range settings {
		settingsMap[setting.Key] = setting.Value
	}

	c.JSON(http.StatusOK, gin.H{
		"data": settingsMap,
	})
}

// UpdateSettingsRequest 批量更新系统设置请求
type UpdateSettingsRequest struct {
	Settings map[string]string `json:"settings" binding:"required"`
}

// UpdateSettings 批量更新系统设置
// PUT /admin/api/settings
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	if err := h.settingService.ValidateSettingsPatch(c.Request.Context(), req.Settings); err != nil {
		var validationErr *config.SettingValidationError
		var conflictErr *config.SettingConflictError
		switch {
		case errors.As(err, &validationErr):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid setting value",
				"message": validationErr.Error(),
			})
			return
		case errors.As(err, &conflictErr):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Conflicting settings",
				"message": conflictErr.Error(),
			})
			return
		}
	}

	// 批量更新设置
	for key, value := range req.Settings {
		if err := h.settingService.Set(c.Request.Context(), key, value); err != nil {
			var validationErr *config.SettingValidationError
			if errors.As(err, &validationErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid setting value",
					"message": validationErr.Error(),
				})
				return
			}
			h.logger.Error("更新系统配置异常",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update settings",
				"message": err.Error(),
			})
			return
		}
	}

	h.logger.Info("系统配置已更新",
		slog.Int("count", len(req.Settings)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Settings updated successfully",
	})
}

// GetSetting 获取单个设置
// GET /admin/api/settings/:key
func (h *SettingsHandler) GetSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Setting key is required",
		})
		return
	}

	setting, err := h.systemSettingRepo.GetByKey(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Setting not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": setting,
	})
}

// UpdateSettingRequest 更新单个设置请求
type UpdateSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

// UpdateSetting 更新单个设置
// PUT /admin/api/settings/:key
func (h *SettingsHandler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Setting key is required",
		})
		return
	}

	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	if err := h.settingService.ValidateSettingsPatch(c.Request.Context(), map[string]string{key: req.Value}); err != nil {
		var validationErr *config.SettingValidationError
		var conflictErr *config.SettingConflictError
		switch {
		case errors.As(err, &validationErr):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid setting value",
				"message": validationErr.Error(),
			})
			return
		case errors.As(err, &conflictErr):
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Conflicting settings",
				"message": conflictErr.Error(),
			})
			return
		}
	}

	if err := h.settingService.Set(c.Request.Context(), key, req.Value); err != nil {
		var validationErr *config.SettingValidationError
		if errors.As(err, &validationErr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid setting value",
				"message": validationErr.Error(),
			})
			return
		}
		var conflictErr *config.SettingConflictError
		if errors.As(err, &conflictErr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Conflicting settings",
				"message": conflictErr.Error(),
			})
			return
		}
		h.logger.Error("更新系统配置异常",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update setting",
			"message": err.Error(),
		})
		return
	}

	h.logger.Info("系统配置已更新",
		slog.String("key", key),
		slog.String("value", safeSettingValueForAdminLog(key, req.Value)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Setting updated successfully",
	})
}

// TestNotificationRequest 测试通知渠道请求
type TestNotificationRequest struct {
	Channel          string `json:"channel" binding:"required"`
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`
}

// TestNotification 测试通知渠道
// POST /admin/api/settings/notifications/test
func (h *SettingsHandler) TestNotification(c *gin.Context) {
	if h.notificationSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "notification service is not initialized",
		})
		return
	}

	var req TestNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
		return
	}

	switch req.Channel {
	case models.NotificationChannelTelegram:
		if err := h.notificationSvc.TestTelegram(c.Request.Context(), req.TelegramBotToken, req.TelegramChatID); err != nil {
			h.logger.Warn("测试 Telegram 通知失败",
				slog.String("error", err.Error()),
			)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   "Failed to send test notification",
				"message": err.Error(),
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Unsupported notification channel",
			"message": "通知渠道当前仅支持 telegram",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "test notification sent successfully",
	})
}

// RegisterRoutes 注册路由
func (h *SettingsHandler) RegisterRoutes(router *gin.RouterGroup) {
	settings := router.Group("/settings")
	{
		settings.GET("", h.GetAllSettings)
		settings.PUT("", h.UpdateSettings)
		settings.POST("/notifications/test", h.TestNotification)
		settings.GET("/:key", h.GetSetting)
		settings.PUT("/:key", h.UpdateSetting)
	}
}

func safeSettingValueForAdminLog(key, value string) string {
	switch key {
	case models.SettingSecurityJWTSecret,
		models.SettingNotificationTelegramBotToken:
		if value == "" {
			return ""
		}
		return "***"
	default:
		return value
	}
}
