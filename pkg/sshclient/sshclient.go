package sshclient

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

// Config represents the YAML configuration
type Config struct {
	Commands []string `yaml:"commands"`
}

// Hosts represents the list of target hosts
type Hosts struct {
	Hosts []string `yaml:"hosts"`
}

// LoadConfig loads the command configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data: %v", err)
	}
	return cfg, nil
}

// LoadHosts loads the list of hosts from a YAML file
func LoadHosts(path string) (*Hosts, error) {
	hosts := &Hosts{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %v", err)
	}
	if err := yaml.Unmarshal(data, hosts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hosts data: %v", err)
	}
	return hosts, nil
}

// GetSSHClient retrieves the SSH client configuration using decrypted credentials
func GetSSHClient() (*ssh.ClientConfig, error) {
	// Get the decryption password from the environment
	decryptionPassword := os.Getenv("CREDENTIALS_PASSWORD")
	if decryptionPassword == "" {
		return nil, fmt.Errorf("CREDENTIALS_PASSWORD environment variable not set")
	}

	// Decrypt the credentials using the function from credentials.go
	creds, err := DecryptCredentials(decryptionPassword, "config/credentials.enc")
	if err != nil {
		return nil, fmt.Errorf("error decrypting credentials: %v", err)
	}

	// Retrieve the SSH user and password from the decrypted credentials
	user := creds["SSH_EXECUTOR_USER"]
	password := creds["SSH_EXECUTOR_PASSWORD"]

	if user == "" || password == "" {
		return nil, fmt.Errorf("SSH_EXECUTOR_USER or SSH_EXECUTOR_PASSWORD not found in decrypted credentials")
	}

	// Create and return the SSH client configuration
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Consider using a proper host key callback
		Timeout:         5 * time.Second,
	}

	return config, nil
}

// ExecuteCommands connects to a host and executes the provided commands
func ExecuteCommands(host string, config *ssh.ClientConfig, commands []string) error {
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %v", host, err)
	}
	defer func(conn *ssh.Client) {
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close connection to %s: %v", host, err)
		}
	}(conn)

	for _, cmd := range commands {
		session, err := conn.NewSession()
		if err != nil {
			log.Printf("Failed to create session for %s: %v", host, err)
			continue
		}

		output, err := session.CombinedOutput(cmd)
		if err != nil {
			log.Printf("Failed to execute command '%s' on %s: %v", cmd, host, err)
		} else {
			log.Printf("Output from %s:\n%s", host, output)
		}

		if err := session.Close(); err != nil {
			log.Printf("Failed to close session for %s: %v", host, err)
		}
	}

	return nil
}
