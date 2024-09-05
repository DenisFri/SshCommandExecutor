package sshclient

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// DecryptCredentials decrypts the encrypted credentials using AES-256 in Go
func DecryptCredentials(password, filePath string) (map[string]string, error) {
	key := []byte(password) // The password used for encryption (must be 32 bytes)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted credentials file: %v", err)
	}

	// Decode the hex-encoded ciphertext
	ciphertext, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %v", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	// Convert the decrypted data into a string (assuming it's key=value pairs)
	plaintext := string(ciphertext)
	return parseCredentials(plaintext)
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
