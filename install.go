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

func getWindowsInstallDir() (string, error) {
	// Try user's local bin directory first
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return "", fmt.Errorf("USERPROFILE environment variable not set")
	}

	// Create a bin directory in the user's AppData\Local folder
	installDir := filepath.Join(userProfile, "AppData", "Local", "bin")

	// Ensure the directory exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create install directory: %v", err)
	}

	return installDir, nil
}

func addToWindowsPath(installDir string) error {
	// Get current PATH
	path := os.Getenv("PATH")
	if strings.Contains(path, installDir) {
		return nil // Already in PATH
	}

	// Add to PATH using setx
	cmd := exec.Command("setx", "PATH", path+";"+installDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update PATH: %v", err)
	}

	return nil
}

func main() {
	// Define flags
	installClsp := flag.Bool("clsp", false, "Build and install only clsp (if --hub is not provided, both are installed)")
	installHub := flag.Bool("hub", false, "Build and install only clsp-hub (if --clsp is not provided, both are installed)")
	flag.Parse()

	// If neither flag is provided, install both (default behavior).
	installBoth := !(*installClsp || *installHub)

	// Determine install directory
	var installDir string
	var err error
	if runtime.GOOS == "windows" {
		installDir, err = getWindowsInstallDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error determining install directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		installDir = "/usr/local/bin"
	}

	fmt.Printf("Installing to: %s\n", installDir)

	// Build binaries in a temporary directory first
	tempDir, err := os.MkdirTemp("", "clsp-install-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory when done

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

	// Determine binary names for final installation
	clspPath := filepath.Join(installDir, getBinaryName("clsp"))
	clspHubPath := filepath.Join(installDir, getBinaryName("clsp-hub"))

	// Remove old binaries if they exist
	if *installClsp || installBoth {
		os.Remove(clspPath)
	}
	if *installHub || installBoth {
		os.Remove(clspHubPath)
	}

	// Copy the built binaries to the install directory
	if *installClsp || installBoth {
		srcPath := filepath.Join(tempDir, getBinaryName("clsp"))
		if err := copyFile(srcPath, clspPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install clsp: %v\n", err)
			os.Exit(1)
		}
	}
	if *installHub || installBoth {
		srcPath := filepath.Join(tempDir, getBinaryName("clsp-hub"))
		if err := copyFile(srcPath, clspHubPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install clsp-hub: %v\n", err)
			os.Exit(1)
		}
	}

	// Set executable permissions (on Unix-like systems)
	if runtime.GOOS != "windows" {
		if *installClsp || installBoth {
			os.Chmod(clspPath, 0755)
		}
		if *installHub || installBoth {
			os.Chmod(clspHubPath, 0755)
		}
	}

	fmt.Printf("\nCLSP binaries installed successfully to %s\n", installDir)

	// Handle Windows PATH
	if runtime.GOOS == "windows" {
		if err := addToWindowsPath(installDir); err != nil {
			fmt.Printf("\nWarning: Failed to update PATH: %v\n", err)
			fmt.Printf("\nTo use the 'clsp' command, you need to add this directory to your PATH:\n")
			fmt.Printf("  %s\n", installDir)
			fmt.Printf("\nYou can do this by:\n")
			fmt.Printf("1. Opening System Properties (Win + Pause/Break)\n")
			fmt.Printf("2. Click 'Advanced system settings'\n")
			fmt.Printf("3. Click 'Environment Variables'\n")
			fmt.Printf("4. Under 'User variables', find and select 'Path'\n")
			fmt.Printf("5. Click 'Edit' and add the above directory\n")
			fmt.Printf("6. Click 'OK' on all windows\n")
			fmt.Printf("7. Restart your terminal\n")
		} else {
			fmt.Printf("\nPATH updated successfully. Please restart your terminal for the changes to take effect.\n")
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
