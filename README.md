# CLSP - Command Line Secure Protocol

CLSP is a secure, command-line messaging tool that provides end-to-end encrypted communication between users. It uses a hub-based architecture where messages are routed through a central server but remain encrypted throughout the process.

## Features

- End-to-end encryption using RSA and AES
- Command-line interface for easy automation
- Hub server for message routing and storage
- User aliases for easier addressing
- Message expiration and cleanup
- File attachment support
- Online status tracking
- Message search and filtering
- TLS support for secure communication

## Installation

### Prerequisites

- Go 1.21 or later
- SQLite3 (for the hub server)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/mattd/clsp.git
cd clsp

# Build the hub server
go build -o clsp-hub ./cmd/clsp-hub

# Build the client
go build -o clsp ./cmd/clsp
```

## Quick Start

1. Start the hub server:
   ```bash
   ./clsp-hub -port 8080 -db .clsp/hub.db
   ```

2. Initialize a user (on each client):
   ```bash
   ./clsp init "Your Name"
   ```

3. Send a message:
   ```bash
   ./clsp send "Recipient Name" "Your message"
   ```

4. List messages:
   ```bash
   ./clsp list
   ```

## Usage

### Hub Server

```bash
clsp-hub [options]

Options:
  -port int     Port to listen on (default 8080)
  -db string    Path to database file (default ".clsp/hub.db")
```

### Client Commands

```bash
clsp <command> [options]

Commands:
  init          Initialize user identity
  send          Send a message
  list          List messages
  status        Check message status
  users         List users
  config        Manage configuration

Configuration options:
  --show              Show current configuration
  --set-hub <url>     Set hub URL
  --set-tls           Enable TLS
  --set-cert <path>   Set TLS certificate path
  --set-expiry <dur>  Set message expiry duration
  --add-alias <a=id>  Add user alias
  --remove-alias <a>  Remove user alias
```

## Security

- Messages are encrypted using RSA for key exchange and AES for message encryption
- Private keys are stored locally and never transmitted
- Messages are stored encrypted on the hub
- TLS support for secure communication
- Message expiration for automatic cleanup

## Architecture

CLSP uses a hub-based architecture:

1. **Hub Server** (`clsp-hub`):
   - Routes messages between users
   - Stores encrypted messages
   - Manages user registration
   - Handles message expiration

2. **Client** (`clsp`):
   - Manages user identity
   - Handles message encryption/decryption
   - Provides command-line interface
   - Stores local configuration

## Configuration

The client configuration is stored in `~/.clsp/config.json` and includes:
- Hub URL
- User ID and display name
- TLS settings
- Message expiry duration
- User aliases

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 