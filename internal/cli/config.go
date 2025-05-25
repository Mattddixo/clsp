package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/mattd/clsp/internal/paths"
)

// Config represents the client configuration
type Config struct {
	HubURL        string            `json:"hub_url"`
	HubRetryCount int               `json:"hub_retry_count"`
	HubRetryDelay time.Duration     `json:"hub_retry_delay"`
	UseTLS        bool              `json:"use_tls"`
	TLSCertPath   string            `json:"tls_cert_path,omitempty"`
	MessageExpiry time.Duration     `json:"message_expiry"`
	UserID        string            `json:"user_id"`
	DisplayName   string            `json:"display_name"`
	UserAliases   map[string]string `json:"user_aliases"`
	LastSyncTime  time.Time         `json:"last_sync_time"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		HubURL:        "http://localhost:8080",
		HubRetryCount: 3,
		HubRetryDelay: 1 * time.Second,
		UseTLS:        false,
		MessageExpiry: 30 * 24 * time.Hour, // 30 days
		UserID:        "",
		DisplayName:   "",
		UserAliases:   make(map[string]string),
		LastSyncTime:  time.Now(),
	}
}

// LoadConfig loads the configuration from file
func LoadConfig() (*Config, error) {
	configPath := paths.GetConfigPath("config.json")

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := SaveConfig(config); err != nil {
			return nil, fmt.Errorf("failed to create default config: %v", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to file
func SaveConfig(config *Config) error {
	if err := paths.EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	configPath := paths.GetConfigPath("config.json")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %v", err)
	}

	return nil
}

// UpdateHubURL updates the hub URL in the configuration
func (c *Config) UpdateHubURL(urlStr string) error {
	// Validate URL format
	if _, err := url.Parse(urlStr); err != nil {
		return fmt.Errorf("invalid hub URL: %v", err)
	}
	c.HubURL = urlStr
	return nil
}

// AddUserAlias adds a user alias to the configuration
func (c *Config) AddUserAlias(alias, userID string) {
	if c.UserAliases == nil {
		c.UserAliases = make(map[string]string)
	}
	c.UserAliases[alias] = userID
}

// GetUserIDByAlias returns the user ID for a given alias
func GetUserIDByAlias(alias string) (string, bool) {
	config, err := LoadConfig()
	if err != nil {
		return "", false
	}

	userID, exists := config.UserAliases[alias]
	return userID, exists
}
