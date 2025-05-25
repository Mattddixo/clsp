package crypto

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// AESKeySize is the size of the AES key in bytes
	AESKeySize = 32
)

// Message represents an encrypted message with metadata
type Message struct {
	ID           string      `json:"id"`
	Sender       string      `json:"sender"`
	Recipient    string      `json:"recipient"`
	Timestamp    int64       `json:"timestamp"`
	Status       string      `json:"status"`
	EncryptedKey []byte      `json:"encrypted_key"`
	IV           []byte      `json:"iv"`
	Content      []byte      `json:"content"`
	Signature    []byte      `json:"signature"`
	Attachment   *Attachment `json:"attachment,omitempty"`
}

// Attachment represents an encrypted file attachment
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Content     []byte `json:"content"`
}

// EncryptMessage encrypts a message for a recipient using their public key
func EncryptMessage(senderPrivateKey *rsa.PrivateKey, recipientPublicKey *rsa.PublicKey, content []byte, attachment *Attachment) (*Message, error) {
	// Generate random AES key
	aesKey := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		return nil, fmt.Errorf("failed to generate AES key: %v", err)
	}

	// Encrypt AES key with recipient's public key
	encryptedKey, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		recipientPublicKey,
		aesKey,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt AES key: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %v", err)
	}

	// Encrypt content
	stream := cipher.NewCTR(block, iv)
	encryptedContent := make([]byte, len(content))
	stream.XORKeyStream(encryptedContent, content)

	// If there's an attachment, encrypt it
	if attachment != nil {
		attachmentContent := make([]byte, len(attachment.Content))
		stream.XORKeyStream(attachmentContent, attachment.Content)
		attachment.Content = attachmentContent
	}

	// Create message
	msg := &Message{
		EncryptedKey: encryptedKey,
		IV:           iv,
		Content:      encryptedContent,
		Attachment:   attachment,
	}

	// Sign message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %v", err)
	}

	hash := sha256.New()
	hash.Write(msgBytes)
	signature, err := rsa.SignPKCS1v15(
		rand.Reader,
		senderPrivateKey,
		crypto.SHA256,
		hash.Sum(nil),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %v", err)
	}

	msg.Signature = signature
	return msg, nil
}

// DecryptMessage decrypts a message using the recipient's private key
func DecryptMessage(recipientPrivateKey *rsa.PrivateKey, msg *Message) ([]byte, error) {
	// Decrypt AES key
	aesKey, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		recipientPrivateKey,
		msg.EncryptedKey,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt AES key: %v", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	// Decrypt content
	stream := cipher.NewCTR(block, msg.IV)
	decryptedContent := make([]byte, len(msg.Content))
	stream.XORKeyStream(decryptedContent, msg.Content)

	// If there's an attachment, decrypt it
	if msg.Attachment != nil {
		attachmentContent := make([]byte, len(msg.Attachment.Content))
		stream.XORKeyStream(attachmentContent, msg.Attachment.Content)
		msg.Attachment.Content = attachmentContent
	}

	return decryptedContent, nil
}

// VerifySignature verifies the message signature using the sender's public key
func VerifySignature(senderPublicKey *rsa.PublicKey, msg *Message) error {
	// Create a copy of the message without the signature
	msgCopy := *msg
	msgCopy.Signature = nil

	// Marshal the message
	msgBytes, err := json.Marshal(msgCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	// Verify signature
	hash := sha256.New()
	hash.Write(msgBytes)
	err = rsa.VerifyPKCS1v15(
		senderPublicKey,
		crypto.SHA256,
		hash.Sum(nil),
		msg.Signature,
	)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %v", err)
	}

	return nil
}
