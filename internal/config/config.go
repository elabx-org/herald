package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port     int    `yaml:"port"`
	DBPath   string `yaml:"db_path"`
	LogLevel string `yaml:"log_level"`
	Cache    Cache  `yaml:"cache"`
	Audit    Audit  `yaml:"audit"`
	Auth     Auth   `yaml:"auth"`
	Komodo   Komodo `yaml:"komodo"`
}

type Cache struct {
	Key               string `yaml:"key"`
	DataPath          string `yaml:"data_path"`
	DefaultTTLSecs    int    `yaml:"default_ttl_seconds"`
	PollIntervalSecs  int    `yaml:"poll_interval_seconds"`
	PollThresholdSecs int    `yaml:"poll_threshold_seconds"`
	OutPathPrefix     string `yaml:"out_path_prefix"`
}

type Audit struct {
	Enabled       bool `yaml:"enabled"`
	RetentionDays int  `yaml:"retention_days"`
}

type Auth struct {
	JWTSecret        string `yaml:"jwt_secret"`
	JWTExpiryHours   int    `yaml:"jwt_expiry_hours"`
	OIDCIssuer       string `yaml:"oidc_issuer"`
	OIDCClientID     string `yaml:"oidc_client_id"`
	OIDCClientSecret string `yaml:"oidc_client_secret"`
}

type Komodo struct {
	URL       string `yaml:"url"`
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	applyEnv(cfg)
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Port:     8765,
		DBPath:   "/data/herald.db",
		LogLevel: "info",
		Cache: Cache{
			DataPath:          "/data/cache.db",
			DefaultTTLSecs:    3600,
			PollIntervalSecs:  600,
			PollThresholdSecs: 300,
		},
		Audit: Audit{RetentionDays: 30},
		Auth:  Auth{JWTExpiryHours: 24},
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("HERALD_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("HERALD_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("HERALD_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("HERALD_CACHE_KEY"); v != "" {
		cfg.Cache.Key = v
	}
	if v := os.Getenv("HERALD_CACHE_DATA_PATH"); v != "" {
		cfg.Cache.DataPath = v
	}
	if v := os.Getenv("HERALD_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("HERALD_AUDIT_ENABLED"); v == "true" || v == "1" {
		cfg.Audit.Enabled = true
	}
	if v := os.Getenv("KOMODO_URL"); v != "" {
		cfg.Komodo.URL = v
	}
	if v := os.Getenv("KOMODO_API_KEY"); v != "" {
		cfg.Komodo.APIKey = v
	}
	if v := os.Getenv("KOMODO_API_SECRET"); v != "" {
		cfg.Komodo.APISecret = v
	}
}
