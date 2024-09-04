package sshclient

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DecryptCredentials decrypts an encrypted file using OpenSSL
func DecryptCredentials(password, filePath string) (map[string]string, error) {
	cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-d", "-in", filePath, "-k", password)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %v", err)
	}

	return parseCredentials(out.String())
}

// parseCredentials parses the decrypted credentials into a map
func parseCredentials(content string) (map[string]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	creds := make(map[string]string)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid credentials format")
		}
		creds[parts[0]] = parts[1]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading credentials: %v", err)
	}

	return creds, nil
}
