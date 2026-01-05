package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Whois         WhoisConfig         `yaml:"whois"`
	Monitor       MonitorConfig       `yaml:"monitor"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"` // debug/release
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type     string `yaml:"type"` // sqlite/mysql/postgres
	Path     string `yaml:"path"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// WhoisConfig represents WHOIS API configuration
type WhoisConfig struct {
	APIURL  string `yaml:"api_url"`
	Timeout string `yaml:"timeout"`
}

// MonitorConfig represents monitoring configuration
type MonitorConfig struct {
	CheckInterval string `yaml:"check_interval"` // Cron expression
	AlertDays     []int  `yaml:"alert_days"`
}

// NotificationsConfig represents notification configuration
type NotificationsConfig struct {
	Email     EmailConfig     `yaml:"email"`
	Webhook   WebhookConfig   `yaml:"webhook"`
	Telegram  TelegramConfig  `yaml:"telegram"`
	DingDing  DingDingConfig  `yaml:"dingding"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled  bool     `yaml:"enabled"`
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	From     string   `yaml:"from"`
	Password string   `yaml:"password"`
	To       []string `yaml:"to"`
}

// WebhookConfig represents webhook notification configuration
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

// TelegramConfig represents Telegram notification configuration
type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

// DingDingConfig represents DingTalk notification configuration
type DingDingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Webhook string `yaml:"webhook"`
	Secret  string `yaml:"secret"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
