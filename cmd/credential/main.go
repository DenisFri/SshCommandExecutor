package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

func encryptCredentials(password []byte, inputPath, outputPath string) error {
	// Read the input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Generate a random 16-byte salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %v", err)
	}

	// Derive a 32-byte key from the password using PBKDF2
	key := pbkdf2.Key(password, salt, 100000, 32, sha256.New)

	// Create the AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Create GCM mode (Galois/Counter Mode) for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Generate a random nonce (GCM standard nonce size is 12 bytes)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Encrypt and authenticate the plaintext
	// GCM automatically appends authentication tag to the ciphertext
	encryptedData := gcm.Seal(nil, nonce, plaintext, nil)

	// Create final format: salt (16) + nonce (12) + encrypted data + auth tag (16, included in encryptedData)
	ciphertext := make([]byte, 16+len(nonce)+len(encryptedData))
	copy(ciphertext[:16], salt)
	copy(ciphertext[16:16+len(nonce)], nonce)
	copy(ciphertext[16+len(nonce):], encryptedData)

	// Convert the ciphertext to hex for easier storage
	hexEncoded := hex.EncodeToString(ciphertext)

	// Write the hex-encoded ciphertext to the output file
	if err := os.WriteFile(outputPath, []byte(hexEncoded), 0600); err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	return nil
}

func main() {
	// Define command-line flags
	encrypt := flag.Bool("encrypt", false, "Encrypt credentials")
	inputFile := flag.String("input", "", "Input file path")
	outputFile := flag.String("output", "", "Output file path")
	flag.Parse()

	if !*encrypt || *inputFile == "" || *outputFile == "" {
		fmt.Println("Usage: credential-tool -encrypt -input=credentials.txt -output=credentials.enc")
		os.Exit(1)
	}

	// Get the password from environment variable
	password := os.Getenv("CREDENTIALS_PASSWORD")
	if password == "" {
		fmt.Println("Error: CREDENTIALS_PASSWORD environment variable not set")
		os.Exit(1)
	}

	// Encrypt the credentials
	if *encrypt {
		if err := encryptCredentials([]byte(password), *inputFile, *outputFile); err != nil {
			log.Fatalf("Error encrypting credentials: %v", err)
		}
		fmt.Printf("Credentials encrypted and saved to %s\n", *outputFile)
	}
}
