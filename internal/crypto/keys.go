package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattd/clsp/internal/paths"
)

const (
	KeySize = 2048
)

// GenerateKeyPair generates a new RSA key pair
func GenerateKeyPair() (privateKey *rsa.PrivateKey, publicKeyPEM []byte, err error) {
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	publicKeyPEM, err = PublicKeyToPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert public key to PEM: %v", err)
	}

	return privateKey, publicKeyPEM, nil
}

// SavePrivateKey saves a private key to a file
func SavePrivateKey(privateKey *rsa.PrivateKey, path string) error {
	// Ensure the key directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create key directory: %v", err)
	}
	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}
	defer outFile.Close()
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}
	if err := pem.Encode(outFile, pemBlock); err != nil {
		return fmt.Errorf("failed to encode private key")
	}
	return nil
}

// LoadPrivateKey loads the private key from disk
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}
	return priv, nil
}

// LoadPublicKey loads the public key from disk
func LoadPublicKey() (*rsa.PublicKey, error) {
	publicKeyPEM, err := os.ReadFile(paths.GetKeyPath("public.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %v", err)
	}

	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode public key PEM")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return rsaPublicKey, nil
}

// LoadPublicKeyFromPEM loads a public key from PEM format
func LoadPublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode public key PEM")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return rsaPublicKey, nil
}

// PublicKeyToPEM converts an RSA public key to PEM format
func PublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return publicKeyPEM, nil
}
