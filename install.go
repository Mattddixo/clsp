package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func getInstallPath() (string, error) {
	// Get the appropriate installation directory based on OS
	var installDir string
	if runtime.GOOS == "windows" {
		// On Windows, use %LOCALAPPDATA%\Programs\clsp
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA environment variable not set")
		}
		installDir = filepath.Join(localAppData, "Programs", "clsp")
	} else {
		// On Unix-like systems, use /usr/local/bin
		installDir = "/usr/local/bin"
	}

	// Create the installation directory if it doesn't exist
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create installation directory: %v", err)
	}

	return installDir, nil
}

func installBinaries() error {
	// Get installation path
	installDir, err := getInstallPath()
	if err != nil {
		return err
	}

	// Build both binaries
	fmt.Println("Building CLSP binaries...")

	// Build clsp
	cmd := exec.Command("go", "build", "-o", "clsp", "./cmd/clsp")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build clsp: %v", err)
	}

	// Build clsp-hub
	cmd = exec.Command("go", "build", "-o", "clsp-hub", "./cmd/clsp-hub")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build clsp-hub: %v", err)
	}

	// Move binaries to installation directory
	clspPath := filepath.Join(installDir, "clsp")
	if runtime.GOOS == "windows" {
		clspPath += ".exe"
	}

	clspHubPath := filepath.Join(installDir, "clsp-hub")
	if runtime.GOOS == "windows" {
		clspHubPath += ".exe"
	}

	// Remove existing binaries if they exist
	os.Remove(clspPath)
	os.Remove(clspHubPath)

	// Move the new binaries
	if err := os.Rename("clsp", clspPath); err != nil {
		return fmt.Errorf("failed to install clsp: %v", err)
	}
	if err := os.Rename("clsp-hub", clspHubPath); err != nil {
		return fmt.Errorf("failed to install clsp-hub: %v", err)
	}

	// Make binaries executable on Unix-like systems
	if runtime.GOOS != "windows" {
		os.Chmod(clspPath, 0755)
		os.Chmod(clspHubPath, 0755)
	}

	fmt.Printf("\nCLSP binaries installed successfully to %s\n", installDir)
	fmt.Println("\nYou can now use 'clsp' and 'clsp-hub' commands from anywhere!")

	if runtime.GOOS == "windows" {
		fmt.Println("\nNote: You may need to restart your terminal for the PATH changes to take effect.")
	}

	return nil
}

func main() {
	if err := installBinaries(); err != nil {
		fmt.Printf("Installation failed: %v\n", err)
		os.Exit(1)
	}
}
