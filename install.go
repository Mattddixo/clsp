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
	// Define flags so that if neither --clsp nor --hub is provided, both are installed (default behavior).
	// If --clsp is provided (and --hub is not), then only clsp is installed.
	// If --hub is provided (and --clsp is not), then only clsp-hub is installed.
	installClsp := flag.Bool("clsp", false, "Build and install only clsp (if --hub is not provided, both are installed)")
	installHub := flag.Bool("hub", false, "Build and install only clsp-hub (if --clsp is not provided, both are installed)")
	flag.Parse()

	// If neither flag is provided, install both (default behavior).
	installBoth := !(*installClsp || *installHub)

	// Determine install directory (e.g. /usr/local/bin on Unix, %LOCALAPPDATA%\Programs\clsp on Windows)
	var installDir string
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			fmt.Fprintln(os.Stderr, "LOCALAPPDATA environment variable not set")
			os.Exit(1)
		}
		installDir = filepath.Join(localAppData, "Programs", "clsp")
	} else {
		installDir = "/usr/local/bin"
	}

	// Ensure install directory exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create install directory %s: %v\n", installDir, err)
		os.Exit(1)
	}

	fmt.Println("Building CLSP binaries...")

	// Build clsp (if --clsp is provided or if neither flag is provided (installBoth))
	if *installClsp || installBoth {
		cmd := exec.Command("go", "build", "-o", "clsp", "./cmd/clsp")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build clsp: %v\n", err)
			os.Exit(1)
		}
	}

	// Build clsp-hub (if --hub is provided or if neither flag is provided (installBoth))
	if *installHub || installBoth {
		cmd := exec.Command("go", "build", "-o", "clsp-hub", "./cmd/clsp-hub")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build clsp-hub: %v\n", err)
			os.Exit(1)
		}
	}

	// Determine binary names (append .exe on Windows)
	clspPath := filepath.Join(installDir, "clsp")
	if runtime.GOOS == "windows" {
		clspPath += ".exe"
	}
	clspHubPath := filepath.Join(installDir, "clsp-hub")
	if runtime.GOOS == "windows" {
		clspHubPath += ".exe"
	}

	// Remove old binaries (if any) so that os.Rename works
	if *installClsp || installBoth {
		os.Remove(clspPath)
	}
	if *installHub || installBoth {
		os.Remove(clspHubPath)
	}

	// Move (rename) the built binaries into the install directory
	if *installClsp || installBoth {
		if err := os.Rename("clsp", clspPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to install clsp: %v\n", err)
			os.Exit(1)
		}
	}
	if *installHub || installBoth {
		if err := os.Rename("clsp-hub", clspHubPath); err != nil {
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
