package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func getInstallDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// Use LOCALAPPDATA\Programs\clsp as the installation directory
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", fmt.Errorf("LOCALAPPDATA environment variable not set")
		}
		installDir := filepath.Join(localAppData, "Programs", "clsp")
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create install directory: %v", err)
		}
		return installDir, nil
	case "darwin", "linux":
		// Use /usr/local/bin for Unix-like systems
		installDir := "/usr/local/bin"
		// Check if we have write permissions
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return "", fmt.Errorf("failed to access %s: %v\nPlease run with sudo for system-wide installation", installDir, err)
		}
		return installDir, nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func updatePath(installDir string) error {
	switch runtime.GOOS {
	case "windows":
		// Get current PATH from registry
		cmd := exec.Command("powershell", "-Command", "[Environment]::GetEnvironmentVariable('Path', 'User')")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get current PATH: %v", err)
		}
		currentPath := strings.TrimSpace(string(output))

		// Check if directory is already in PATH
		if strings.Contains(currentPath, installDir) {
			return nil // Already in PATH
		}

		// Add to PATH using PowerShell and update current session
		psCmd := fmt.Sprintf(`
			$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
			$newPath = $userPath + ';%s'
			[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
			$env:Path = $newPath
		`, installDir)
		cmd = exec.Command("powershell", "-Command", psCmd)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update PATH: %v", err)
		}

		// Verify the update
		cmd = exec.Command("powershell", "-Command", "$env:Path")
		output, err = cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to verify PATH update: %v", err)
		}
		newPath := strings.TrimSpace(string(output))
		if !strings.Contains(newPath, installDir) {
			return fmt.Errorf("PATH update verification failed")
		}
		return nil

	case "darwin", "linux":
		// On Unix-like systems, /usr/local/bin is typically already in PATH
		// Just verify it exists and is writable
		if err := os.MkdirAll(installDir, 0755); err != nil {
			return fmt.Errorf("failed to access %s: %v\nPlease run with sudo for system-wide installation", installDir, err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func verifyInstallation(installDir string) bool {
	binaryName := "clsp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(installDir, binaryName)

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return false
	}

	// Try to run the binary
	cmd := exec.Command(binaryPath, "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func main() {
	// Define flags
	installClsp := flag.Bool("clsp", false, "Build and install only clsp (if --hub is not provided, both are installed)")
	installHub := flag.Bool("hub", false, "Build and install only clsp-hub (if --clsp is not provided, both are installed)")
	flag.Parse()

	// If neither flag is provided, install both (default behavior)
	installBoth := !(*installClsp || *installHub)

	// Determine install directory
	installDir, err := getInstallDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining install directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installing to: %s\n", installDir)

	// Build binaries in a temporary directory first
	tempDir, err := os.MkdirTemp("", "clsp-install-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("Building CLSP binaries...")

	// Helper function to get binary name with extension
	getBinaryName := func(name string) string {
		if runtime.GOOS == "windows" {
			return name + ".exe"
		}
		return name
	}

	// Build clsp
	if *installClsp || installBoth {
		outputPath := filepath.Join(tempDir, getBinaryName("clsp"))
		cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/clsp")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build clsp: %v\n", err)
			os.Exit(1)
		}
	}

	// Build clsp-hub
	if *installHub || installBoth {
		outputPath := filepath.Join(tempDir, getBinaryName("clsp-hub"))
		cmd := exec.Command("go", "build", "-o", outputPath, "./cmd/clsp-hub")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build clsp-hub: %v\n", err)
			os.Exit(1)
		}
	}

	// Remove old binaries if they exist
	if *installClsp || installBoth {
		os.RemoveAll(filepath.Join(installDir, getBinaryName("clsp")))
	}
	if *installHub || installBoth {
		os.RemoveAll(filepath.Join(installDir, getBinaryName("clsp-hub")))
	}

	// Copy the built binaries to the install directory
	if *installClsp || installBoth {
		srcPath := filepath.Join(tempDir, getBinaryName("clsp"))
		if err := copyFile(srcPath, filepath.Join(installDir, getBinaryName("clsp"))); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install clsp: %v\n", err)
			os.Exit(1)
		}
	}
	if *installHub || installBoth {
		srcPath := filepath.Join(tempDir, getBinaryName("clsp-hub"))
		if err := copyFile(srcPath, filepath.Join(installDir, getBinaryName("clsp-hub"))); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install clsp-hub: %v\n", err)
			os.Exit(1)
		}
	}

	// Set executable permissions (on Unix-like systems)
	if runtime.GOOS != "windows" {
		if *installClsp || installBoth {
			os.Chmod(filepath.Join(installDir, "clsp"), 0755)
		}
		if *installHub || installBoth {
			os.Chmod(filepath.Join(installDir, "clsp-hub"), 0755)
		}
	}

	fmt.Printf("\nCLSP binaries installed successfully to %s\n", installDir)

	// Update PATH
	if err := updatePath(installDir); err != nil {
		fmt.Printf("\nWarning: Failed to update PATH: %v\n", err)
		fmt.Printf("\nTo use the commands, you need to add this directory to your PATH:\n")
		fmt.Printf("  %s\n", installDir)

		if runtime.GOOS == "windows" {
			fmt.Printf("\nOn Windows, run this command in PowerShell:\n")
			fmt.Printf("  $env:Path = [Environment]::GetEnvironmentVariable('Path', 'User')\n")
		} else {
			fmt.Printf("\nOn Unix-like systems, add this line to your shell's rc file (e.g., ~/.bashrc, ~/.zshrc):\n")
			fmt.Printf("  export PATH=\"$PATH:%s\"\n", installDir)
		}
	} else {
		fmt.Printf("\nPATH updated successfully.\n")

		// Verify installation
		if verifyInstallation(installDir) {
			fmt.Printf("Installation verified successfully.\n")
			if runtime.GOOS == "windows" {
				fmt.Printf("\nThe command should be available in this PowerShell session.\n")
				fmt.Printf("If not, run: $env:Path = [Environment]::GetEnvironmentVariable('Path', 'User')\n")
			} else {
				fmt.Printf("\nPlease restart your terminal for the changes to take effect.\n")
			}
			fmt.Printf("Then try: clsp --version\n")
		} else {
			fmt.Printf("\nWarning: Installation verification failed. Please try running:\n")
			fmt.Printf("  %s%sclsp%s --version\n",
				installDir,
				string(filepath.Separator),
				map[string]string{"windows": ".exe"}[runtime.GOOS])
		}
	}

	if (*installClsp && *installHub) || installBoth {
		fmt.Println("\nYou can now use 'clsp' and 'clsp-hub' commands from anywhere!")
	} else if *installClsp {
		fmt.Println("\nYou can now use the 'clsp' command from anywhere!")
	} else if *installHub {
		fmt.Println("\nYou can now use the 'clsp-hub' command from anywhere!")
	} else {
		fmt.Println("\nNo binary installed (--clsp and --hub are both false).")
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Read the source file
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}

	// Write to the destination file
	if err := os.WriteFile(dst, srcData, 0755); err != nil {
		return fmt.Errorf("failed to write destination file: %v", err)
	}

	return nil
}
