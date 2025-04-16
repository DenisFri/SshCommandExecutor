package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

func encryptCredentials(password []byte, inputPath, outputPath string) error {
	// Ensure password is exactly 32 bytes (AES-256)
	if len(password) != 32 {
		var keyBytes [32]byte
		copy(keyBytes[:], password)
		password = keyBytes[:]
	}

	// Read the input file
	plaintext, err := ioutil.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Create the AES cipher block
	block, err := aes.NewCipher(password)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Create a random IV
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return fmt.Errorf("failed to generate IV: %v", err)
	}

	// Encrypt the plaintext
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// Convert the ciphertext to hex for easier storage
	hexEncoded := hex.EncodeToString(ciphertext)

	// Write the hex-encoded ciphertext to the output file
	if err := ioutil.WriteFile(outputPath, []byte(hexEncoded), 0600); err != nil {
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
