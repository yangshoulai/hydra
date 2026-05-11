package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yangshoulai/hydra/internal/models"
	"github.com/yangshoulai/hydra/internal/service/config"
)

const (
	defaultSendTimeout        = 10 * time.Second
	maxMessageLength          = 3800
	maxFieldValueLength       = 220
	maxRenderedFieldLineCount = 12
)

// Field 表示通知消息中的一行结构化字段。
type Field struct {
	Name  string
	Value string
}

// Event 表示一次业务通知事件。
type Event struct {
	Type      string
	Title     string
	Fields    []Field
	CreatedAt time.Time
}

type telegramConfig struct {
	BotToken string
	ChatID   string
}

type runtimeConfig struct {
	Enabled  bool
	Channel  string
	Events   map[string]bool
	Telegram telegramConfig
}

// Service 统一封装系统通知能力。
//
// 当前仅实现 Telegram，但配置结构按 channel + events + channel-specific config
// 拆分，后续接入其它渠道时只需要扩展发送器，不需要重做触发点。
type Service struct {
	logger         *slog.Logger
	settingService *config.SettingService
	client         *http.Client
}

// NewService 创建通知服务。
func NewService(logger *slog.Logger, settingService *config.SettingService) *Service {
	return &Service{
		logger:         logger,
		settingService: settingService,
		client: &http.Client{
			Timeout: defaultSendTimeout,
		},
	}
}

// NotifyAsync 异步发送通知，避免登录、代理重试等主流程被通知渠道阻塞。
func (s *Service) NotifyAsync(event Event) {
	if s == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaultSendTimeout)
		defer cancel()

		if err := s.Notify(ctx, event); err != nil {
			s.logger.Warn("发送通知失败",
				slog.String("event", event.Type),
				slog.String("error", err.Error()),
			)
		}
	}()
}

// Notify 根据当前系统设置判断并发送通知。
func (s *Service) Notify(ctx context.Context, event Event) error {
	if s == nil {
		return nil
	}

	cfg := s.loadConfig(ctx)
	if !cfg.shouldSend(event.Type) {
		return nil
	}

	switch cfg.Channel {
	case models.NotificationChannelTelegram:
		return s.sendTelegram(ctx, cfg.Telegram, formatMessage(event))
	default:
		return nil
	}
}

// TestTelegram 使用用户输入的临时配置发送测试消息，不依赖已保存设置。
func (s *Service) TestTelegram(ctx context.Context, botToken string, chatID string) error {
	if s == nil {
		return fmt.Errorf("通知服务未初始化")
	}
	tg := telegramConfig{
		BotToken: strings.TrimSpace(botToken),
		ChatID:   strings.TrimSpace(chatID),
	}
	if !tg.ready() {
		return fmt.Errorf("Telegram Bot Token 和 Chat ID 均不能为空")
	}

	return s.sendTelegram(ctx, tg, formatMessage(Event{
		Type:  "test",
		Title: "测试通知",
		Fields: []Field{
			{Name: "结果", Value: "Hydra 已成功连接 Telegram 通知渠道"},
		},
		CreatedAt: time.Now(),
	}))
}

func (s *Service) loadConfig(ctx context.Context) runtimeConfig {
	eventsJSON := s.settingService.GetString(ctx, models.SettingNotificationEvents, "[]")
	events := make([]string, 0)
	if strings.TrimSpace(eventsJSON) != "" {
		if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
			s.logger.Warn("解析通知发送配置失败",
				slog.String("error", err.Error()),
			)
		}
	}

	eventsSet := make(map[string]bool, len(events))
	for _, event := range events {
		eventsSet[event] = true
	}

	return runtimeConfig{
		Enabled: s.settingService.GetBool(ctx, models.SettingNotificationEnabled, false),
		Channel: strings.TrimSpace(s.settingService.GetString(
			ctx,
			models.SettingNotificationChannel,
			models.NotificationChannelTelegram,
		)),
		Events: eventsSet,
		Telegram: telegramConfig{
			BotToken: strings.TrimSpace(s.settingService.GetString(ctx, models.SettingNotificationTelegramBotToken, "")),
			ChatID:   strings.TrimSpace(s.settingService.GetString(ctx, models.SettingNotificationTelegramChatID, "")),
		},
	}
}

func (cfg runtimeConfig) shouldSend(eventType string) bool {
	if !cfg.Enabled {
		return false
	}
	if !cfg.Events[eventType] {
		return false
	}
	switch cfg.Channel {
	case models.NotificationChannelTelegram:
		return cfg.Telegram.ready()
	default:
		return false
	}
}

func (cfg telegramConfig) ready() bool {
	return strings.TrimSpace(cfg.BotToken) != "" && strings.TrimSpace(cfg.ChatID) != ""
}

func (s *Service) sendTelegram(ctx context.Context, cfg telegramConfig, text string) error {
	if !cfg.ready() {
		return fmt.Errorf("Telegram 配置未完成")
	}

	endpoint := url.URL{
		Scheme: "https",
		Host:   "api.telegram.org",
		Path:   "/bot" + cfg.BotToken + "/sendMessage",
	}
	body, err := json.Marshal(map[string]any{
		"chat_id":                  cfg.ChatID,
		"text":                     text,
		"parse_mode":               "MarkdownV2",
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("构造 Telegram 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建 Telegram 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用 Telegram API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram API 返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("解析 Telegram 响应失败: %w", err)
	}
	if !parsed.OK {
		desc := strings.TrimSpace(parsed.Description)
		if desc == "" {
			desc = "unknown error"
		}
		return fmt.Errorf("Telegram API 返回失败: %s", desc)
	}

	return nil
}

func formatMessage(event Event) string {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = notificationEventLabel(event.Type)
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	lines := []string{
		"🔔 *Hydra 通知*",
		"━━━━━━━━━━━━",
		"📌 *事件*：" + escapeTelegramMarkdownV2(title),
		"🕒 *时间*：" + escapeTelegramMarkdownV2(event.CreatedAt.Format("2006-01-02 15:04:05")),
	}

	renderedFields := make([]string, 0, len(event.Fields))
	for _, field := range event.Fields {
		if len(renderedFields) >= maxRenderedFieldLineCount {
			break
		}
		name := singleLine(strings.TrimSpace(field.Name))
		value := strings.TrimSpace(field.Value)
		if name == "" || value == "" {
			continue
		}
		renderedFields = append(renderedFields,
			"• *"+escapeTelegramMarkdownV2(name)+"*："+escapeTelegramMarkdownV2(truncate(value, maxFieldValueLength)),
		)
	}
	if len(renderedFields) > 0 {
		lines = append(lines, "", "📋 *详情*")
		lines = append(lines, renderedFields...)
	}

	lines = append(lines, "", "✅ _Hydra 已按当前通知策略推送_")

	return truncateEscapedMarkdownV2(strings.Join(lines, "\n"), maxMessageLength)
}

func notificationEventLabel(eventType string) string {
	switch eventType {
	case models.NotificationEventCircuitBreaker:
		return "代理渠道或密钥熔断"
	case models.NotificationEventAdminLogin:
		return "管理员登录"
	case models.NotificationEventAdminPasswordChange:
		return "管理员修改密码"
	default:
		return eventType
	}
}

func truncate(value string, maxLen int) string {
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.Join(strings.Fields(value), " ")
}

// escapeTelegramMarkdownV2 转义 Telegram MarkdownV2 要求的特殊字符。
//
// 需要转义的字符：_ * [ ] ( ) ~ ` > # + - = | { } . ! 以及反斜杠自身。
// 所有业务字段和时间字符串都必须先经过该函数，再拼入带 Markdown 语法的模板。
func escapeTelegramMarkdownV2(value string) string {
	var builder strings.Builder
	builder.Grow(len(value) + 8)
	for _, r := range value {
		switch r {
		case '_', '*', '[', ']', '(', ')', '~', '`', '>', '#', '+', '-', '=', '|', '{', '}', '.', '!', '\\':
			builder.WriteRune('\\')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func truncateEscapedMarkdownV2(value string, maxLen int) string {
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}

	cutRunes := runes[:maxLen-1]
	for hasOddTrailingBackslashes(string(cutRunes)) && len(cutRunes) > 0 {
		cutRunes = cutRunes[:len(cutRunes)-1]
	}
	return string(cutRunes) + "…"
}

func hasOddTrailingBackslashes(value string) bool {
	count := 0
	for i := len(value) - 1; i >= 0 && value[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}
