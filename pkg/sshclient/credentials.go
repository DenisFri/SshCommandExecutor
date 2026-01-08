package sshclient

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// DecryptCredentials decrypts the encrypted credentials using AES-256-GCM
// The password is derived using PBKDF2 with a salt extracted from the ciphertext
func DecryptCredentials(password, filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted credentials file: %v", err)
	}

	// Decode the hex-encoded ciphertext
	ciphertext, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %v", err)
	}

	// Extract salt (first 16 bytes), nonce (next 12 bytes), and encrypted data
	// Minimum size: 16 (salt) + 12 (nonce) + 16 (auth tag) = 44 bytes
	if len(ciphertext) < 44 {
		return nil, fmt.Errorf("ciphertext too short (must be at least 44 bytes for salt+nonce+tag)")
	}

	salt := ciphertext[:16]
	nonce := ciphertext[16:28]
	encryptedData := ciphertext[28:]

	// Derive a 32-byte key from the password using PBKDF2
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// Create the AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Create GCM mode for authenticated decryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials (wrong password or corrupted data): %v", err)
	}

	// Convert the decrypted data into a string (assuming it's key=value pairs)
	return parseCredentials(string(plaintext))
}

// parseCredentials parses the decrypted credentials into a map
func parseCredentials(content string) (map[string]string, error) {
	lines := strings.Split(content, "\n")
	creds := make(map[string]string)
	for _, line := range lines {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid credentials format")
		}
		creds[parts[0]] = parts[1]
	}
	return creds, nil
}
