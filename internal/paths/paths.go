package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the name of the application
	AppName = "clsp"
	// ConfigDirName is the name of the config directory
	ConfigDirName = ".clsp"
)

var (
	// HomeDir is the user's home directory
	HomeDir string
	// ConfigDir is the path to the global config directory
	ConfigDir string
	// KeyDir is the path to the keys directory
	KeyDir string
	// HubDBPath is the path to the hub database
	HubDBPath string
)

func init() {
	var err error
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get user home dir: %v", err))
	}
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			panic("LOCALAPPDATA environment variable not set")
		}
		ConfigDir = filepath.Join(localAppData, AppName)
	} else {
		ConfigDir = filepath.Join(HomeDir, ".config", AppName)
	}
	KeyDir = filepath.Join(ConfigDir, "keys")
	HubDBPath = filepath.Join(ConfigDir, "hub.db")
}

// EnsureConfigDir ensures that the config directory exists
func EnsureConfigDir() error {
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	if err := os.MkdirAll(KeyDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %v", err)
	}
	return nil
}

// GetConfigPath returns the path to a config file
func GetConfigPath(filename string) string {
	return filepath.Join(ConfigDir, filename)
}

// GetKeyPath returns the path to a key file
func GetKeyPath(filename string) string {
	return filepath.Join(KeyDir, filename)
}
