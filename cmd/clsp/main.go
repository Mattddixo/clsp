package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattd/clsp/internal/cli"
)

func printUsage() {
	fmt.Println("CLSP - Command Line Secure Protocol")
	fmt.Println("\nFirst time setup:")
	fmt.Println("  clsp install                    Install and create initial configuration")
	fmt.Println("\nUsage:")
	fmt.Println("  clsp init <display-name>        Initialize user identity")
	fmt.Println("  clsp send <recipient> <message> Send a message")
	fmt.Println("  clsp list                       List messages")
	fmt.Println("  clsp status <message-id>        Check message status")
	fmt.Println("  clsp users                      List users")
	fmt.Println("  clsp config                     Manage configuration")
	fmt.Println("\nConfiguration options:")
	fmt.Println("  clsp config --show              Show current configuration")
	fmt.Println("  clsp config --set-hub <url>     Set hub URL")
	fmt.Println("  clsp config --set-tls           Enable TLS")
	fmt.Println("  clsp config --set-cert <path>   Set TLS certificate path")
	fmt.Println("  clsp config --set-expiry <dur>  Set message expiry duration")
	fmt.Println("  clsp config --add-alias <a=id>  Add user alias")
	fmt.Println("  clsp config --remove-alias <a>  Remove user alias")
	fmt.Println("\nUse 'clsp <command> --help' for more information about a command")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Check if installed for all commands except install
	if command != "install" && !cli.IsInstalled() {
		fmt.Println("CLSP is not installed. Please run 'clsp install' first to set up your configuration.")
		fmt.Println("This will create the necessary configuration files in your home directory.")
		os.Exit(1)
	}

	switch command {
	case "install":
		if cli.IsInstalled() {
			fmt.Println("CLSP is already installed. Use 'clsp config' to modify your configuration.")
			os.Exit(1)
		}
		if err := cli.Install(); err != nil {
			fmt.Printf("Installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Installation completed successfully!")
		fmt.Println("\nNext steps:")
		fmt.Println("1. Configure your hub connection (if needed):")
		fmt.Println("   clsp config --set-hub https://your-hub:8080")
		fmt.Println("2. Initialize your identity:")
		fmt.Println("   clsp init \"Your Name\"")
		return

	case "init":
		if len(args) > 0 {
			fmt.Println("Note: Display name will be prompted interactively")
			fmt.Println("Any additional arguments will be ignored")
		}

		if err := cli.InitUser(); err != nil {
			fmt.Printf("Error initializing user: %v\n", err)
			os.Exit(1)
		}

	case "send":
		sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
		attachment := sendCmd.String("attachment", "", "Path to attachment file")
		recipient := sendCmd.String("to", "", "Recipient display name or alias")
		message := sendCmd.String("message", "", "Message content")

		sendCmd.Parse(args)

		if *recipient == "" || *message == "" {
			if len(sendCmd.Args()) >= 2 {
				*recipient = sendCmd.Args()[0]
				*message = strings.Join(sendCmd.Args()[1:], " ")
			} else {
				fmt.Println("Error: recipient and message required")
				sendCmd.PrintDefaults()
				os.Exit(1)
			}
		}

		if err := cli.SendMessage(*recipient, *message, *attachment); err != nil {
			fmt.Printf("Error sending message: %v\n", err)
			os.Exit(1)
		}

	case "list":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		unreadOnly := listCmd.Bool("unread", false, "Show only unread messages")
		limit := listCmd.Int("limit", 0, "Limit number of messages shown")
		search := listCmd.String("search", "", "Search messages by content")

		listCmd.Parse(args)

		if err := cli.ListMessages(*unreadOnly, *limit, *search); err != nil {
			fmt.Printf("Error listing messages: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if len(args) < 1 {
			fmt.Println("Error: message ID required")
			os.Exit(1)
		}
		if err := cli.MessageStatus(args[0]); err != nil {
			fmt.Printf("Error checking message status: %v\n", err)
			os.Exit(1)
		}

	case "users":
		usersCmd := flag.NewFlagSet("users", flag.ExitOnError)
		onlineOnly := usersCmd.Bool("online", false, "Show only online users")
		search := usersCmd.String("search", "", "Search users by name")

		usersCmd.Parse(args)

		if err := cli.ListUsers(*onlineOnly, *search); err != nil {
			fmt.Printf("Error listing users: %v\n", err)
			os.Exit(1)
		}

	case "config":
		configCmd := flag.NewFlagSet("config", flag.ExitOnError)
		show := configCmd.Bool("show", false, "Show current configuration")
		setHub := configCmd.String("set-hub", "", "Set hub URL")
		setTLS := configCmd.Bool("set-tls", false, "Enable/disable TLS")
		setCert := configCmd.String("set-cert", "", "Set TLS certificate path")
		setExpiry := configCmd.String("set-expiry", "", "Set message expiry duration (e.g., '24h', '7d')")
		addAlias := configCmd.String("add-alias", "", "Add user alias (format: alias=userid)")
		removeAlias := configCmd.String("remove-alias", "", "Remove user alias")

		if err := configCmd.Parse(args); err != nil {
			fmt.Printf("Error parsing config flags: %v\n", err)
			os.Exit(1)
		}

		config, err := cli.LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		modified := false

		if *show {
			fmt.Printf("Hub URL: %s\n", config.HubURL)
			fmt.Printf("Use TLS: %v\n", config.UseTLS)
			if config.UseTLS && config.TLSCertPath != "" {
				fmt.Printf("TLS Certificate: %s\n", config.TLSCertPath)
			}
			fmt.Printf("Message Expiry: %v\n", config.MessageExpiry)
			fmt.Printf("User Aliases:\n")
			for alias, id := range config.UserAliases {
				fmt.Printf("  %s -> %s\n", alias, id)
			}
			return
		}

		if *setHub != "" {
			if err := config.UpdateHubURL(*setHub); err != nil {
				fmt.Printf("Error updating hub URL: %v\n", err)
				os.Exit(1)
			}
			modified = true
		}

		if configCmd.Parsed() {
			if *setTLS {
				config.UseTLS = true
				modified = true
			}

			if *setCert != "" {
				config.TLSCertPath = *setCert
				modified = true
			}

			if *setExpiry != "" {
				duration, err := time.ParseDuration(*setExpiry)
				if err != nil {
					fmt.Printf("Invalid duration format: %v\n", err)
					os.Exit(1)
				}
				config.MessageExpiry = duration
				modified = true
			}

			if *addAlias != "" {
				parts := strings.Split(*addAlias, "=")
				if len(parts) != 2 {
					fmt.Println("Invalid alias format. Use: alias=userid")
					os.Exit(1)
				}
				config.AddUserAlias(parts[0], parts[1])
				modified = true
			}

			if *removeAlias != "" {
				delete(config.UserAliases, *removeAlias)
				modified = true
			}
		}

		if modified {
			if err := cli.SaveConfig(config); err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Configuration updated successfully")
		} else if !*show {
			fmt.Println("No changes made to configuration")
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}
