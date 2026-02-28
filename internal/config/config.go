package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIToken string `yaml:"-"` // from HERALD_API_TOKEN env

	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`

	Providers []ProviderConfig `yaml:"providers"`

	Komodo struct {
		URL       string `yaml:"url"`
		APIKey    string `yaml:"api_key"`
		APISecret string `yaml:"api_secret"`
	} `yaml:"komodo"`

	Cache struct {
		DefaultPolicy string `yaml:"default_policy"`
		DefaultTTL    int    `yaml:"default_ttl"`
		EncryptionKey string `yaml:"encryption_key"`
		DataPath      string `yaml:"data_path"`
	} `yaml:"cache"`

	Audit struct {
		Enabled       bool   `yaml:"enabled"`
		Path          string `yaml:"path"`
		RetentionDays int    `yaml:"retention_days"`
	} `yaml:"audit"`

	Alerts struct {
		TokenExpiryWarningDays int `yaml:"token_expiry_warning_days"`
	} `yaml:"alerts"`

	StacksRepo struct {
		Path         string `yaml:"path"`
		Remote       string `yaml:"remote"`
		Branch       string `yaml:"branch"`
		PollInterval int    `yaml:"poll_interval"`
	} `yaml:"stacks_repo"`
}

type ProviderConfig struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Token    string `yaml:"token"`
	Priority int    `yaml:"priority"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}

	// Defaults
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8765
	cfg.Cache.DefaultPolicy = "memory"
	cfg.Cache.DefaultTTL = 3600
	cfg.Cache.DataPath = "/data/cache.db"
	cfg.Audit.RetentionDays = 30
	cfg.Alerts.TokenExpiryWarningDays = 7

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Env overrides
	if v := os.Getenv("HERALD_API_TOKEN"); v != "" {
		cfg.APIToken = v
	}
	if v := os.Getenv("OP_CONNECT_TOKEN"); v != "" {
		for i := range cfg.Providers {
			if cfg.Providers[i].Type == "connect_server" {
				cfg.Providers[i].Token = v
			}
		}
	}
	if v := os.Getenv("OP_SERVICE_ACCOUNT_TOKEN"); v != "" {
		found := false
		for i := range cfg.Providers {
			if cfg.Providers[i].Type == "service_account" {
				cfg.Providers[i].Token = v
				found = true
			}
		}
		// Auto-create a default service account provider if none configured
		if !found {
			cfg.Providers = append(cfg.Providers, ProviderConfig{
				Name:     "1password",
				Type:     "service_account",
				Token:    v,
				Priority: 1,
			})
		}
	}
	if v := os.Getenv("KOMODO_API_KEY"); v != "" {
		cfg.Komodo.APIKey = v
	}
	if v := os.Getenv("KOMODO_API_SECRET"); v != "" {
		cfg.Komodo.APISecret = v
	}
	if v := os.Getenv("HERALD_CACHE_KEY"); v != "" {
		cfg.Cache.EncryptionKey = v
	}
	if v := os.Getenv("HERALD_CACHE_DATA_PATH"); v != "" {
		cfg.Cache.DataPath = v
	}

	return cfg, nil
}
