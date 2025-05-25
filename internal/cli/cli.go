package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mattd/clsp/internal/crypto"
	"github.com/mattd/clsp/internal/paths"
)

const (
	// HubURL is the default URL for the hub server
	HubURL = "http://localhost:8080"
)

// User represents a CLSP user
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	PublicKey   string `json:"public_key"`
}

// HubInfo represents the hub's configuration and status
type HubInfo struct {
	Status string
	Config struct {
		MessageExpiry time.Duration `json:"message_expiry"`
		UseTLS        bool          `json:"use_tls"`
		TLSCertPath   string        `json:"tls_cert_path,omitempty"`
		RateLimit     int           `json:"rate_limit"`
		HubTimeout    time.Duration `json:"hub_timeout"`
		HubRetryCount int           `json:"hub_retry_count"`
		HubRetryDelay time.Duration `json:"hub_retry_delay"`
	}
}

// CheckHubHealth checks if the hub is available and returns its configuration
func CheckHubHealth(hubURL string) (*HubInfo, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(hubURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("hub not reachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub returned status %d", resp.StatusCode)
	}

	var info HubInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse hub response: %v", err)
	}

	return &info, nil
}

// CheckUsername checks if a username is available on the hub
func CheckUsername(hubURL, username string) (bool, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(fmt.Sprintf("%s/check-username?username=%s", hubURL, url.QueryEscape(username)))
	if err != nil {
		return false, fmt.Errorf("failed to check username: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("hub returned status %d", resp.StatusCode)
	}

	var result struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	return result.Available, nil
}

// InitUser initializes a new user identity interactively
func InitUser() error {
	// Check if user is already initialized
	config, err := LoadConfig()
	if err == nil && config.UserID != "" {
		fmt.Print("A user is already initialized. Do you want to reinitialize? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			return fmt.Errorf("user initialization cancelled")
		}

		// Clean up old configuration
		fmt.Println("Cleaning up old configuration...")
		if err := cleanupOldConfig(); err != nil {
			return fmt.Errorf("failed to clean up old configuration: %v", err)
		}
	}

	// Prompt for hub URL
	defaultHub := "http://localhost:8080"
	fmt.Printf("Hub URL [%s]: ", defaultHub)
	var hubURL string
	fmt.Scanln(&hubURL)
	if hubURL == "" {
		hubURL = defaultHub
	}

	// Validate hub URL
	if _, err := url.Parse(hubURL); err != nil {
		return fmt.Errorf("invalid hub URL: %v", err)
	}

	// Check hub health
	fmt.Println("Checking hub connection...")
	hubInfo, err := CheckHubHealth(hubURL)
	if err != nil {
		return fmt.Errorf("hub not available: %v", err)
	}
	fmt.Println("Hub connection successful!")

	// Show hub configuration
	fmt.Printf("\nHub Configuration:\n")
	fmt.Printf("Message Expiry: %v\n", hubInfo.Config.MessageExpiry)
	fmt.Printf("Rate Limit: %d messages/minute\n", hubInfo.Config.RateLimit)
	if hubInfo.Config.UseTLS {
		fmt.Println("TLS: Enabled")
		if hubInfo.Config.TLSCertPath != "" {
			fmt.Printf("TLS Certificate: %s\n", hubInfo.Config.TLSCertPath)
		}
	} else {
		fmt.Println("TLS: Disabled")
	}

	// Get display name
	var displayName string
	for {
		fmt.Print("\nChoose a display name: ")
		fmt.Scanln(&displayName)
		if displayName == "" {
			fmt.Println("Display name cannot be empty")
			continue
		}

		// Check if username is available
		available, err := CheckUsername(hubURL, displayName)
		if err != nil {
			return fmt.Errorf("failed to check username: %v", err)
		}
		if !available {
			fmt.Println("This display name is already taken")
			continue
		}
		break
	}

	// Generate key pair
	fmt.Println("\nGenerating encryption keys...")
	privateKey, publicKeyPEM, err := crypto.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate keys: %v", err)
	}

	// Create user ID
	userID := uuid.New().String()

	// Save local configuration
	config = &Config{
		HubURL:       hubURL,
		UserID:       userID,
		DisplayName:  displayName,
		UserAliases:  make(map[string]string),
		LastSyncTime: time.Now(),
	}

	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	// Save private key
	if err := crypto.SavePrivateKey(privateKey, paths.GetKeyPath("private.key")); err != nil {
		return fmt.Errorf("failed to save private key: %v", err)
	}

	// Register with hub
	fmt.Println("Registering with hub...")
	reqBody, err := json.Marshal(map[string]string{
		"user_id":      userID,
		"display_name": displayName,
		"public_key":   string(publicKeyPEM),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}

	client := &http.Client{
		Timeout: hubInfo.Config.HubTimeout,
	}
	resp, err := client.Post(hubURL+"/register", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to register with hub: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("hub returned status %d", resp.StatusCode)
	}

	fmt.Println("Registration successful!")
	fmt.Printf("\nYour user ID: %s\n", userID)
	fmt.Printf("Display name: %s\n", displayName)
	fmt.Println("\nYou can now start sending messages!")

	return nil
}

// cleanupOldConfig removes old configuration files and keys
func cleanupOldConfig() error {
	// Remove old private key
	if err := os.Remove(paths.GetKeyPath("private.key")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove old private key: %v", err)
	}

	// Remove old config files
	configFiles := []string{"config.json", "user.json"}
	for _, file := range configFiles {
		if err := os.Remove(paths.GetConfigPath(file)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old %s: %v", file, err)
		}
	}

	return nil
}

// SendMessage sends an encrypted message to a recipient
func SendMessage(recipient, message, attachmentPath string) error {
	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Get hub configuration to get timeout
	hubInfo, err := CheckHubHealth(config.HubURL)
	if err != nil {
		return fmt.Errorf("failed to get hub configuration: %v", err)
	}

	// Load private key
	privateKey, err := crypto.LoadPrivateKey(paths.GetKeyPath("private.key"))
	if err != nil {
		return fmt.Errorf("failed to load private key: %v", err)
	}

	// Get recipient's public key
	client := &http.Client{
		Timeout: hubInfo.Config.HubTimeout,
	}
	resp, err := client.Get(config.HubURL + "/users")
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return fmt.Errorf("failed to decode users: %v", err)
	}

	var recipientUser *User
	for _, u := range users {
		if u.DisplayName == recipient {
			recipientUser = &u
			break
		}
	}

	if recipientUser == nil {
		return fmt.Errorf("recipient not found: %s", recipient)
	}

	// Load recipient's public key
	recipientPublicKey, err := crypto.LoadPublicKeyFromPEM([]byte(recipientUser.PublicKey))
	if err != nil {
		return fmt.Errorf("failed to load recipient's public key: %v", err)
	}

	// Handle attachment if provided
	var attachment *crypto.Attachment
	if attachmentPath != "" {
		content, err := os.ReadFile(attachmentPath)
		if err != nil {
			return fmt.Errorf("failed to read attachment: %v", err)
		}

		attachment = &crypto.Attachment{
			Filename:    filepath.Base(attachmentPath),
			ContentType: "application/octet-stream", // TODO: detect content type
			Size:        int64(len(content)),
			Content:     content,
		}
	}

	// Encrypt message
	msg, err := crypto.EncryptMessage(privateKey, recipientPublicKey, []byte(message), attachment)
	if err != nil {
		return fmt.Errorf("failed to encrypt message: %v", err)
	}

	// Set message metadata
	msg.ID = uuid.New().String()
	msg.Sender = config.UserID
	msg.Recipient = recipientUser.ID
	msg.Timestamp = time.Now().Unix()
	msg.Status = "sent"

	// Send message to hub
	reqBody, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err = client.Post(config.HubURL+"/message", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send message: %s", string(body))
	}

	fmt.Printf("Message sent successfully to %s\n", recipient)
	return nil
}

// ListMessages lists received messages with optional filtering
func ListMessages(unreadOnly bool, limit int, search string) error {
	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Get hub configuration to get timeout
	hubInfo, err := CheckHubHealth(config.HubURL)
	if err != nil {
		return fmt.Errorf("failed to get hub configuration: %v", err)
	}

	// Build query parameters
	params := url.Values{}
	params.Set("user_id", config.UserID)
	if unreadOnly {
		params.Set("unread", "true")
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if search != "" {
		params.Set("search", search)
	}

	// Get messages from hub
	client := &http.Client{
		Timeout: hubInfo.Config.HubTimeout,
	}

	resp, err := client.Get(fmt.Sprintf("%s/messages?%s", config.HubURL, params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to get messages: %v", err)
	}
	defer resp.Body.Close()

	var messages []crypto.Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return fmt.Errorf("failed to decode messages: %v", err)
	}

	// Load private key
	privateKey, err := crypto.LoadPrivateKey(paths.GetKeyPath("private.key"))
	if err != nil {
		return fmt.Errorf("failed to load private key: %v", err)
	}

	// Decrypt and display messages
	for _, msg := range messages {
		content, err := crypto.DecryptMessage(privateKey, &msg)
		if err != nil {
			fmt.Printf("Failed to decrypt message %s: %v\n", msg.ID, err)
			continue
		}

		// Format message display
		fmt.Printf("\nMessage ID: %s\n", msg.ID)
		fmt.Printf("From: %s\n", msg.Sender)
		fmt.Printf("Time: %s\n", time.Unix(msg.Timestamp, 0).Format(time.RFC3339))
		fmt.Printf("Status: %s\n", msg.Status)
		fmt.Printf("Message: %s\n", string(content))

		if msg.Attachment != nil {
			fmt.Printf("Attachment: %s (%d bytes)\n", msg.Attachment.Filename, msg.Attachment.Size)
		}
		fmt.Println("---")
	}

	return nil
}

// MessageStatus checks the delivery status of a message
func MessageStatus(messageID string) error {
	// TODO: Implement message status check
	fmt.Println("Message status check not implemented yet")
	return nil
}

// ListUsers lists known users with optional filtering
func ListUsers(onlineOnly bool, search string) error {
	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Get hub configuration to get timeout
	hubInfo, err := CheckHubHealth(config.HubURL)
	if err != nil {
		return fmt.Errorf("failed to get hub configuration: %v", err)
	}

	// Build query parameters
	params := url.Values{}
	if onlineOnly {
		params.Set("online", "true")
	}
	if search != "" {
		params.Set("search", search)
	}

	// Get users from hub
	client := &http.Client{
		Timeout: hubInfo.Config.HubTimeout,
	}

	resp, err := client.Get(fmt.Sprintf("%s/users?%s", config.HubURL, params.Encode()))
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return fmt.Errorf("failed to decode users: %v", err)
	}

	// Display users
	fmt.Println("\nKnown Users:")
	fmt.Println("------------")
	for _, u := range users {
		fmt.Printf("ID: %s\n", u.ID)
		fmt.Printf("Name: %s\n", u.DisplayName)

		// Show alias if exists
		for alias, id := range config.UserAliases {
			if id == u.ID {
				fmt.Printf("Alias: %s\n", alias)
				break
			}
		}
		fmt.Println("---")
	}

	return nil
}
