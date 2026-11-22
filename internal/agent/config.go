package agent

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type Config struct {
	SaasURL       string        `mapstructure:"saas_url"`
	AgentToken    string        `mapstructure:"agent_token"`
	ServerID      string        `mapstructure:"server_id"`
	WebSocketURL  string        `mapstructure:"websocket_url"`
	PollInterval  time.Duration `mapstructure:"poll_interval"`
	StatsInterval time.Duration `mapstructure:"stats_interval"`
	LogLevel      string        `mapstructure:"log_level"`

	// Database credentials for agent operations
	MySQL      DatabaseConfig `mapstructure:"mysql"`
	PostgreSQL DatabaseConfig `mapstructure:"postgresql"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("agent")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/opt/sitekit")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	// Defaults
	viper.SetDefault("poll_interval", "5s")
	viper.SetDefault("stats_interval", "60s")
	viper.SetDefault("log_level", "info")

	// Database defaults
	viper.SetDefault("mysql.host", "localhost")
	viper.SetDefault("mysql.port", 3306)
	viper.SetDefault("mysql.user", "sitekit")
	viper.SetDefault("postgresql.host", "localhost")
	viper.SetDefault("postgresql.port", 5432)
	viper.SetDefault("postgresql.user", "sitekit")

	// Environment variable overrides
	viper.SetEnvPrefix("SITEKIT")
	viper.AutomaticEnv()

	// Allow env vars for sensitive data
	if token := os.Getenv("SITEKIT_AGENT_TOKEN"); token != "" {
		viper.Set("agent_token", token)
	}
	if saasURL := os.Getenv("SITEKIT_SAAS_URL"); saasURL != "" {
		viper.Set("saas_url", saasURL)
	}
	if serverID := os.Getenv("SITEKIT_SERVER_ID"); serverID != "" {
		viper.Set("server_id", serverID)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// Config file not found is OK if env vars are set
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if cfg.AgentToken == "" {
		return nil, fmt.Errorf("agent_token is required")
	}
	if cfg.SaasURL == "" {
		return nil, fmt.Errorf("saas_url is required")
	}
	if err := validateEndpointURL(cfg.SaasURL, "https", "http"); err != nil {
		return nil, fmt.Errorf("invalid saas_url: %w", err)
	}
	if cfg.WebSocketURL != "" {
		if err := validateEndpointURL(cfg.WebSocketURL, "wss", "ws"); err != nil {
			return nil, fmt.Errorf("invalid websocket_url: %w", err)
		}
	}
	cfg.SaasURL = strings.TrimRight(cfg.SaasURL, "/")

	return &cfg, nil
}

func validateEndpointURL(rawURL, secureScheme, loopbackScheme string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if parsed.User != nil {
		return fmt.Errorf("credentials in URL are not allowed")
	}
	if parsed.Host == "" {
		return fmt.Errorf("absolute URL with a host is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("query strings and fragments are not allowed")
	}

	if parsed.Scheme == secureScheme {
		return nil
	}
	if parsed.Scheme != loopbackScheme {
		return fmt.Errorf("%s is required", secureScheme)
	}

	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "localhost" {
		return nil
	}
	ip := net.ParseIP(hostname)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%s is required outside loopback development endpoints", secureScheme)
	}
	return nil
}
