package hub

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattd/clsp/internal/crypto"
	"github.com/mattd/clsp/internal/paths"
	_ "github.com/mattn/go-sqlite3"
)

const (
	// MessageExpiry is the duration after which undelivered messages are deleted
	MessageExpiry = 30 * 24 * time.Hour // 30 days
)

// HubConfig represents the hub's global configuration
type HubConfig struct {
	MessageExpiry time.Duration `json:"message_expiry"`
	UseTLS        bool          `json:"use_tls"`
	TLSCertPath   string        `json:"tls_cert_path,omitempty"`
	RateLimit     int           `json:"rate_limit"` // messages per minute
	HubTimeout    time.Duration `json:"hub_timeout"`
	HubRetryCount int           `json:"hub_retry_count"`
	HubRetryDelay time.Duration `json:"hub_retry_delay"`
}

// Server represents a CLSP hub server
type Server struct {
	port     int
	db       *sql.DB
	server   *http.Server
	stopChan chan struct{}
	mu       sync.RWMutex
	config   HubConfig
}

// User represents a CLSP user
type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	PublicKey   string    `json:"public_key"`
	LastSeen    time.Time `json:"last_seen"`
	Online      bool      `json:"online"`
}

// Message represents a stored message
type Message struct {
	ID          string     `json:"id"`
	SenderID    string     `json:"sender_id"`
	RecipientID string     `json:"recipient_id"`
	Content     []byte     `json:"content"`
	CreatedAt   time.Time  `json:"created_at"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

// NewServer creates a new hub server with default configuration
func NewServer(dbPath string) (*Server, error) {
	// If no dbPath is provided, use the default global path
	if dbPath == "" {
		dbPath = paths.HubDBPath
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	server := &Server{
		db: db,
		config: HubConfig{
			MessageExpiry: 30 * 24 * time.Hour, // 30 days
			UseTLS:        false,
			RateLimit:     60, // 60 messages per minute
			HubTimeout:    10 * time.Second,
			HubRetryCount: 3,
			HubRetryDelay: 1 * time.Second,
		},
		stopChan: make(chan struct{}),
	}

	if err := server.createTables(); err != nil {
		db.Close()
		return nil, err
	}

	return server, nil
}

// Start initializes and starts the hub server
func (s *Server) Start() error {
	// Start cleanup goroutine
	go s.cleanupLoop()

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/config", s.handleConfig)
	mux.HandleFunc("/check-username", s.handleCheckUsername)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/users", s.handleUsers)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/messages", s.handleMessages)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the hub server
func (s *Server) Shutdown() {
	close(s.stopChan)
	if s.server != nil {
		s.server.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
}

// createTables creates the necessary database tables
func (s *Server) createTables() error {
	// Create users table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			public_key TEXT NOT NULL,
			last_seen INTEGER NOT NULL,
			online BOOLEAN NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %v", err)
	}

	// Create messages table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			sender_id TEXT NOT NULL,
			recipient_id TEXT NOT NULL,
			content BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			read_at INTEGER,
			expires_at INTEGER NOT NULL,
			FOREIGN KEY (sender_id) REFERENCES users(id),
			FOREIGN KEY (recipient_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create messages table: %v", err)
	}

	return nil
}

// cleanupLoop periodically cleans up expired messages and updates user online status
func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Delete expired messages
			_, err := s.db.Exec(
				"DELETE FROM messages WHERE expires_at <= ?",
				time.Now().Unix(),
			)
			if err != nil {
				log.Printf("Failed to delete expired messages: %v", err)
			}

			// Update user online status (users inactive for more than 5 minutes are considered offline)
			_, err = s.db.Exec(
				"UPDATE users SET online = 0 WHERE last_seen <= ?",
				time.Now().Add(-5*time.Minute).Unix(),
			)
			if err != nil {
				log.Printf("Failed to update user online status: %v", err)
			}

		case <-s.stopChan:
			return
		}
	}
}

// handleRegister handles user registration
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid user data", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if user.ID == "" || user.DisplayName == "" || user.PublicKey == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Check if display name is taken by another user
	var existingUserID string
	err := s.db.QueryRow("SELECT id FROM users WHERE display_name = ? AND id != ?", user.DisplayName, user.ID).Scan(&existingUserID)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if existingUserID != "" {
		http.Error(w, "Display name already taken", http.StatusConflict)
		return
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check if user exists
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)", user.ID).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		// Update existing user
		_, err = tx.Exec(
			"UPDATE users SET display_name = ?, public_key = ?, last_seen = ?, online = ? WHERE id = ?",
			user.DisplayName,
			user.PublicKey,
			time.Now().Unix(),
			true,
			user.ID,
		)
	} else {
		// Insert new user
		_, err = tx.Exec(
			"INSERT INTO users (id, display_name, public_key, last_seen, online) VALUES (?, ?, ?, ?, ?)",
			user.ID,
			user.DisplayName,
			user.PublicKey,
			time.Now().Unix(),
			true,
		)
	}

	if err != nil {
		http.Error(w, "Failed to store user", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleUsers returns a list of users
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	onlineOnly := r.URL.Query().Get("online") == "true"
	search := r.URL.Query().Get("search")

	// Build query
	query := "SELECT id, display_name, public_key, last_seen, online FROM users"
	args := []interface{}{}
	conditions := []string{}

	if onlineOnly {
		conditions = append(conditions, "online = 1")
	}
	if search != "" {
		conditions = append(conditions, "display_name LIKE ?")
		args = append(args, "%"+search+"%")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Execute query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to query users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		var lastSeenUnix int64
		if err := rows.Scan(&user.ID, &user.DisplayName, &user.PublicKey, &lastSeenUnix, &user.Online); err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			return
		}
		user.LastSeen = time.Unix(lastSeenUnix, 0)
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// handleMessage handles message delivery
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg crypto.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid message", http.StatusBadRequest)
		return
	}

	// Set message expiry
	expiresAt := time.Now().Add(s.config.MessageExpiry)

	// Store message
	_, err := s.db.Exec(
		"INSERT INTO messages (id, sender_id, recipient_id, content, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		msg.ID,
		msg.Sender,
		msg.Recipient,
		msg.Content,
		time.Now().Unix(),
		expiresAt.Unix(),
	)
	if err != nil {
		http.Error(w, "Failed to store message", http.StatusInternalServerError)
		return
	}

	// Update sender's last seen time
	_, err = s.db.Exec(
		"UPDATE users SET last_seen = ?, online = 1 WHERE id = ?",
		time.Now().Unix(),
		msg.Sender,
	)
	if err != nil {
		log.Printf("Failed to update sender's last seen time: %v", err)
	}

	w.WriteHeader(http.StatusCreated)
}

// handleMessages returns messages for a user
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	unreadOnly := r.URL.Query().Get("unread") == "true"
	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return
		}
	}
	search := r.URL.Query().Get("search")

	// Build query
	query := `
		SELECT m.id, m.sender_id, m.recipient_id, m.content, m.created_at, m.read_at, m.expires_at,
			   u.display_name as sender_name
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.recipient_id = ? AND m.expires_at > ?
	`
	args := []interface{}{userID, time.Now().Unix()}

	if unreadOnly {
		query += " AND m.read_at IS NULL"
	}
	if search != "" {
		// Note: This is a simple search. For better search, consider using FTS5
		query += " AND m.content LIKE ?"
		args = append(args, "%"+search+"%")
	}

	query += " ORDER BY m.created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	// Execute query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to query messages", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var createdUnix, expiresUnix int64
		var readUnix sql.NullInt64
		var senderName string
		if err := rows.Scan(
			&msg.ID, &msg.SenderID, &msg.RecipientID, &msg.Content,
			&createdUnix, &readUnix, &expiresUnix, &senderName,
		); err != nil {
			http.Error(w, "Failed to scan message", http.StatusInternalServerError)
			return
		}
		msg.CreatedAt = time.Unix(createdUnix, 0)
		msg.ExpiresAt = time.Unix(expiresUnix, 0)
		if readUnix.Valid {
			readTime := time.Unix(readUnix.Int64, 0)
			msg.ReadAt = &readTime
		}
		messages = append(messages, msg)
	}

	// Mark messages as read
	if !unreadOnly {
		_, err = s.db.Exec(
			"UPDATE messages SET read_at = ? WHERE recipient_id = ? AND read_at IS NULL",
			time.Now().Unix(),
			userID,
		)
		if err != nil {
			log.Printf("Failed to mark messages as read: %v", err)
		}
	}

	// Update user's last seen time
	_, err = s.db.Exec(
		"UPDATE users SET last_seen = ?, online = 1 WHERE id = ?",
		time.Now().Unix(),
		userID,
	)
	if err != nil {
		log.Printf("Failed to update user's last seen time: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check database connection
	if err := s.db.Ping(); err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"config": s.config,
	})
}

// handleConfig handles the hub configuration endpoint
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.config)
}

// handleCheckUsername checks if a username is available
func (s *Server) handleCheckUsername(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE display_name = ?)", username).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"available": !exists,
	})
}

// SetPort sets the port number for the server
func (s *Server) SetPort(port int) {
	s.port = port
}

// SetTimeout sets the hub timeout
func (s *Server) SetTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.HubTimeout = timeout
}

// SetMessageExpiry sets the message expiry duration
func (s *Server) SetMessageExpiry(expiry time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.MessageExpiry = expiry
}

// SetRateLimit sets the rate limit (messages per minute)
func (s *Server) SetRateLimit(limit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.RateLimit = limit
}
