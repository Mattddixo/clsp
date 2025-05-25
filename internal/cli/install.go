package cli

import (
	"fmt"
	"os"

	"github.com/mattd/clsp/internal/paths"
)

// Install performs the initial installation and configuration
func Install() error {
	// Create config directory
	if err := paths.EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Create default config if it doesn't exist
	configPath := paths.GetConfigPath("config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := SaveConfig(config); err != nil {
			return fmt.Errorf("failed to create default config: %v", err)
		}
		fmt.Printf("Created default configuration at %s\n", configPath)
		fmt.Printf("Default hub URL: %s\n", config.HubURL)
		fmt.Println("You can modify these settings using 'clsp config' before initializing your identity")
	}

	return nil
}

// IsInstalled checks if CLSP is properly installed
func IsInstalled() bool {
	configPath := paths.GetConfigPath("config.json")
	_, err := os.Stat(configPath)
	return err == nil
}
