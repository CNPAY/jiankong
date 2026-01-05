package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"domain-monitor/internal/config"
	"domain-monitor/internal/database"
	"domain-monitor/internal/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Notifier interface for different notification types
type Notifier interface {
	Send(domain *models.Domain, daysRemaining int) error
}

// NotifyService handles notifications
type NotifyService struct {
	notifiers []Notifier
}

// NewNotifyService creates a new notification service
func NewNotifyService(cfg *config.NotificationsConfig) *NotifyService {
	service := &NotifyService{
		notifiers: make([]Notifier, 0),
	}

	// Add enabled notifiers
	if cfg.Email.Enabled {
		service.notifiers = append(service.notifiers, NewEmailNotifier(&cfg.Email))
	}

	if cfg.Webhook.Enabled {
		service.notifiers = append(service.notifiers, NewWebhookNotifier(&cfg.Webhook))
	}

	if cfg.Telegram.Enabled {
		service.notifiers = append(service.notifiers, NewTelegramNotifier(&cfg.Telegram))
	}

	if cfg.DingDing.Enabled {
		service.notifiers = append(service.notifiers, NewDingDingNotifier(&cfg.DingDing))
	}

	return service
}

// SendNotification sends notification through all enabled channels
func (s *NotifyService) SendNotification(domain *models.Domain, daysRemaining int) error {
	var lastErr error
	successCount := 0

	for _, notifier := range s.notifiers {
		notifierType := fmt.Sprintf("%T", notifier)
		if err := notifier.Send(domain, daysRemaining); err != nil {
			fmt.Printf("[ERROR] %s notification failed: %v\n", notifierType, err)
			lastErr = err
			// Record failed notification
			s.recordNotification(domain, notifier, "failed")
			continue
		}

		// Record successful notification
		s.recordNotification(domain, notifier, "success")
		successCount++
		fmt.Printf("[SUCCESS] %s notification sent\n", notifierType)
	}

	if successCount > 0 && lastErr != nil {
		// At least one succeeded, don't return error
		return nil
	}

	return lastErr
}

// recordNotification records notification in database
func (s *NotifyService) recordNotification(domain *models.Domain, notifier Notifier, status string) {
	db := database.GetDB()

	notification := &models.Notification{
		DomainID: domain.ID,
		Type:     fmt.Sprintf("%T", notifier),
		Content:  fmt.Sprintf("Domain %s expires in %d days", domain.Name, domain.DaysRemaining),
		Status:   status,
		SentAt:   time.Now(),
	}

	db.Create(notification)
}

// EmailNotifier sends email notifications
type EmailNotifier struct {
	config *config.EmailConfig
}

// NewEmailNotifier creates a new email notifier
func NewEmailNotifier(cfg *config.EmailConfig) *EmailNotifier {
	return &EmailNotifier{config: cfg}
}

// Send sends email notification
func (e *EmailNotifier) Send(domain *models.Domain, daysRemaining int) error {
	// Build email content
	subject := fmt.Sprintf("域名到期提醒：%s 还有 %d 天到期", domain.Name, daysRemaining)

	var statusEmoji string
	if daysRemaining <= 7 {
		statusEmoji = "🔴 紧急"
	} else if daysRemaining <= 30 {
		statusEmoji = "🟡 警告"
	} else {
		statusEmoji = "🟢 正常"
	}

	body := fmt.Sprintf(`
域名到期提醒

状态：%s
域名：%s
剩余天数：%d 天
到期日期：%s
注册商：%s
域名状态：%s
最后检查：%s

请及时续费以避免域名过期！
`,
		statusEmoji,
		domain.Name,
		daysRemaining,
		domain.ExpiryDate.Format("2006-01-02"),
		domain.Registrar,
		domain.Status,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// Build email message
	message := fmt.Sprintf("From: %s\r\n", e.config.From)
	message += fmt.Sprintf("To: %s\r\n", strings.Join(e.config.To, ","))
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "Content-Type: text/plain; charset=UTF-8\r\n"
	message += "\r\n"
	message += body

	// SMTP authentication
	auth := smtp.PlainAuth("", e.config.From, e.config.Password, e.config.SMTPHost)

	// Send email
	addr := fmt.Sprintf("%s:%d", e.config.SMTPHost, e.config.SMTPPort)
	err := smtp.SendMail(addr, auth, e.config.From, e.config.To, []byte(message))
	if err != nil {
		// QQ mail and some other providers return "short response" error
		// but the email is actually sent successfully. Ignore this specific error.
		errMsg := err.Error()
		if !strings.Contains(errMsg, "short response") {
			return fmt.Errorf("failed to send email: %w", err)
		}
		fmt.Printf("[EMAIL] Email sent (ignoring 'short response' error from SMTP server)\n")
	}

	fmt.Printf("[EMAIL] Successfully sent notification for domain %s to %v\n",
		domain.Name, e.config.To)
	return nil
}

// WebhookNotifier sends webhook notifications
type WebhookNotifier struct {
	config *config.WebhookConfig
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(cfg *config.WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{config: cfg}
}

// Send sends webhook notification
func (w *WebhookNotifier) Send(domain *models.Domain, daysRemaining int) error {
	payload := map[string]interface{}{
		"domain":         domain.Name,
		"days_remaining": daysRemaining,
		"expiry_date":    domain.ExpiryDate.Format("2006-01-02"),
		"registrar":      domain.Registrar,
		"status":         domain.Status,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(w.config.URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// TelegramNotifier sends Telegram notifications
type TelegramNotifier struct {
	config *config.TelegramConfig
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(cfg *config.TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{config: cfg}
}

// Send sends Telegram notification
func (t *TelegramNotifier) Send(domain *models.Domain, daysRemaining int) error {
	message := fmt.Sprintf("⚠️ 域名到期提醒\n\nDomain: %s\n剩余天数: %d\n到期日: %s\n注册商: %s",
		domain.Name, daysRemaining, domain.ExpiryDate.Format("2006-01-02"), domain.Registrar)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.config.BotToken)

	payload := map[string]interface{}{
		"chat_id": t.config.ChatID,
		"text":    message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create HTTP client with SOCKS5 proxy support
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Use SOCKS5 proxy (socks5://127.0.0.1:7890)
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:7890", nil, proxy.Direct)
	if err != nil {
		fmt.Printf("[TELEGRAM] Failed to create SOCKS5 proxy: %v\n", err)
	} else {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		client.Transport = transport
		fmt.Printf("[TELEGRAM] Using SOCKS5 proxy: 127.0.0.1:7890\n")
	}

	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// DingDingNotifier sends DingTalk notifications
type DingDingNotifier struct {
	config *config.DingDingConfig
}

// NewDingDingNotifier creates a new DingTalk notifier
func NewDingDingNotifier(cfg *config.DingDingConfig) *DingDingNotifier {
	return &DingDingNotifier{config: cfg}
}

// Send sends DingTalk notification
func (d *DingDingNotifier) Send(domain *models.Domain, daysRemaining int) error {
	// 构建消息文本
	var statusEmoji string
	if daysRemaining <= 7 {
		statusEmoji = "🔴"
	} else if daysRemaining <= 30 {
		statusEmoji = "🟡"
	} else {
		statusEmoji = "🟢"
	}

	message := fmt.Sprintf("## %s 域名到期提醒\n\n"+
		"**域名**: %s\n\n"+
		"**剩余天数**: %d 天\n\n"+
		"**到期日期**: %s\n\n"+
		"**注册商**: %s\n\n"+
		"**状态**: %s",
		statusEmoji,
		domain.Name,
		daysRemaining,
		domain.ExpiryDate.Format("2006-01-02"),
		domain.Registrar,
		domain.Status,
	)

	// 构建请求体
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": "域名到期提醒",
			"text":  message,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// 构建 URL（带签名）
	webhookURL := d.config.Webhook

	// 如果配置了加签密钥，添加签名
	if d.config.Secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := d.generateSign(timestamp, d.config.Secret)

		parsedURL, err := url.Parse(webhookURL)
		if err != nil {
			return fmt.Errorf("invalid webhook URL: %w", err)
		}

		query := parsedURL.Query()
		query.Add("timestamp", timestamp)
		query.Add("sign", sign)
		parsedURL.RawQuery = query.Encode()
		webhookURL = parsedURL.String()
	}

	// 发送请求
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dingding webhook returned status %d", resp.StatusCode)
	}

	// 检查响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if errCode, ok := result["errcode"].(float64); ok && errCode != 0 {
			return fmt.Errorf("dingding API error: %v", result["errmsg"])
		}
	}

	return nil
}

// generateSign 生成钉钉签名
func (d *DingDingNotifier) generateSign(timestamp, secret string) string {
	stringToSign := fmt.Sprintf("%s\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
