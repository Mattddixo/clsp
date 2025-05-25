package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	KeySize = 2048
	KeyDir  = ".clsp"
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
func SavePrivateKey(key *rsa.PrivateKey, path string) error {
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	data := pem.EncodeToMemory(privateKeyPEM)
	if data == nil {
		return fmt.Errorf("failed to encode private key")
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	return nil
}

// LoadPrivateKey loads the private key from disk
func LoadPrivateKey() (*rsa.PrivateKey, error) {
	privateKeyPEM, err := os.ReadFile(filepath.Join(KeyDir, "private.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	return privateKey, nil
}

// LoadPublicKey loads the public key from disk
func LoadPublicKey() (*rsa.PublicKey, error) {
	publicKeyPEM, err := os.ReadFile(filepath.Join(KeyDir, "public.pem"))
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
