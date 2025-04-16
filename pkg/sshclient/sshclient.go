package sshclient

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v2"
)

// ExecutorConfig contains global configuration for the SSH executor
type ExecutorConfig struct {
	// Connection settings
	ConnectTimeout time.Duration
	ExecuteTimeout time.Duration
	KeepAliveTime  time.Duration

	// SSH settings
	UseKnownHosts  bool
	KnownHostsPath string
	DefaultPort    int

	// Execution settings
	OutputDirectory string
	ConcurrentLimit int
}

// DefaultExecutorConfig returns the default configuration
func DefaultExecutorConfig() *ExecutorConfig {
	homePath, _ := os.UserHomeDir()
	knownHostsPath := ""
	if homePath != "" {
		knownHostsPath = filepath.Join(homePath, ".ssh", "known_hosts")
	}

	return &ExecutorConfig{
		ConnectTimeout:  30 * time.Second,
		ExecuteTimeout:  2 * time.Minute,
		KeepAliveTime:   30 * time.Second,
		UseKnownHosts:   true,
		KnownHostsPath:  knownHostsPath,
		DefaultPort:     22,
		OutputDirectory: "output",
		ConcurrentLimit: 10,
	}
}

// Playbook defines a collection of reusable command sequences
type Playbook struct {
	Name     string   `yaml:"name"`
	Commands []string `yaml:"commands"`
}

// PlaybookConfig represents the playbook configuration from the YAML file
type PlaybookConfig struct {
	Playbooks []Playbook `yaml:"playbooks"`
}

// HostConfig represents the host configuration with an assigned playbook
type HostConfig struct {
	Hostname string `yaml:"hostname"`
	Playbook string `yaml:"playbook"`
}

// HostsConfig represents the host assignments in the YAML file
type HostsConfig struct {
	Hosts []HostConfig `yaml:"hosts"`
}

// LoadPlaybooks loads the playbook definitions from a YAML file
func LoadPlaybooks(path string) (*PlaybookConfig, error) {
	playbookConfig := &PlaybookConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook file: %v", err)
	}
	if err := yaml.Unmarshal(data, playbookConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal playbook data: %v", err)
	}
	return playbookConfig, nil
}

// LoadHosts loads the host configuration from a YAML file
func LoadHosts(path string) (*HostsConfig, error) {
	hostsConfig := &HostsConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %v", err)
	}
	if err := yaml.Unmarshal(data, hostsConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hosts data: %v", err)
	}
	return hostsConfig, nil
}

// FindPlaybook finds the playbook by name from the playbook configuration
func FindPlaybook(playbookConfig *PlaybookConfig, playbookName string) (*Playbook, error) {
	for _, playbook := range playbookConfig.Playbooks {
		if playbook.Name == playbookName {
			return &playbook, nil
		}
	}
	return nil, fmt.Errorf("playbook %s not found", playbookName)
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
	keyPath := creds["SSH_KEY_PATH"]

	if user == "" {
		return nil, fmt.Errorf("SSH_EXECUTOR_USER not found in decrypted credentials")
	}

	// Configure authentication methods
	var authMethods []ssh.AuthMethod

	// Add password authentication if available
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	// Add key-based authentication if a key path is provided
	if keyPath != "" {
		// Expand ~ to user's home directory if needed
		if strings.HasPrefix(keyPath, "~") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %v", err)
			}
			keyPath = filepath.Join(homeDir, keyPath[1:])
		}

		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %v", err)
		}

		// Create the signer for the private key
		var signer ssh.Signer
		if passphrase := creds["SSH_KEY_PASSPHRASE"]; passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %v", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Ensure at least one authentication method is available
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available, provide either SSH_EXECUTOR_PASSWORD or SSH_KEY_PATH")
	}

	// Use known_hosts file for host key verification
	homePath, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}

	knownHostsPath := filepath.Join(homePath, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		// Fallback to insecure callback with warning if known_hosts can't be loaded
		log.Printf("Warning: Could not load known_hosts file from %s: %v", knownHostsPath, err)
		log.Printf("Using insecure host key callback. This is NOT recommended for production use")
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// Create and return the SSH client configuration
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	return config, nil
}

// ExecuteCommands connects to a host and executes the provided commands
func ExecuteCommands(host string, config *ssh.ClientConfig, commands []string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a dial context for the SSH connection
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Connect using context-aware dialer
	netConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:22", host))
	if err != nil {
		return fmt.Errorf("failed to dial %s: %v", host, err)
	}

	// Create SSH connection from net.Conn
	clientConn, chans, reqs, err := ssh.NewClientConn(netConn, host, config)
	if err != nil {
		netConn.Close()
		return fmt.Errorf("failed to create SSH client connection for %s: %v", host, err)
	}

	conn := ssh.NewClient(clientConn, chans, reqs)
	defer func(conn *ssh.Client) {
		err := conn.Close()
		if err != nil {
			log.Printf("Failed to close connection to %s: %v", host, err)
		}
	}(conn)

	// Define the output directory
	outputDir := "output"

	// Create the output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// Try to create the directory
		err := os.Mkdir(outputDir, 0755)
		if err != nil {
			log.Printf("Failed to create output directory: %v", err)
			return err
		}
		log.Printf("Output directory created: %s", outputDir)
	}

	// Create the output file for the host
	outputFilePath := filepath.Join(outputDir, fmt.Sprintf("%s_output.txt", host))
	outputFile, err := os.OpenFile(outputFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to create or open output file for host %s: %v", host, err)
		return err
	}
	defer func(outputFile *os.File) {
		err := outputFile.Close()
		if err != nil {
			log.Printf("Failed to close output file for %s: %v", host, err)
		}
	}(outputFile)

	// Write the output of each command to the file
	for _, cmd := range commands {
		// Check if context is done before creating a new session
		select {
		case <-ctx.Done():
			return fmt.Errorf("execution timeout: %v", ctx.Err())
		default:
			// Continue execution
		}

		session, err := conn.NewSession()
		if err != nil {
			log.Printf("Failed to create session for %s: %v", host, err)
			continue
		}
		// Always close the session when done
		defer session.Close()

		// Create a buffer to store the output
		var outputBuf bytes.Buffer
		var errorBuf bytes.Buffer
		session.Stdout = &outputBuf
		session.Stderr = &errorBuf

		// Start an ExecuteCommands with timeout
		errChan := make(chan error, 1)
		go func() {
			errChan <- session.Run(cmd)
		}()

		// Wait for command completion or timeout
		var execErr error
		select {
		case execErr = <-errChan:
			// Command completed
		case <-ctx.Done():
			// Force close the session on timeout
			session.Close()
			execErr = fmt.Errorf("command execution timed out: %v", ctx.Err())
		}

		// Combine output
		output := outputBuf.String()
		errorOutput := errorBuf.String()

		if errorOutput != "" {
			output += "\nStderr: " + errorOutput
		}

		if execErr != nil {
			log.Printf("Failed to execute command '%s' on %s: %v", cmd, host, execErr)
		}

		// Write the output to file
		if _, err := outputFile.WriteString(fmt.Sprintf("Command: %s\nOutput:\n%s\n---\n", cmd, output)); err != nil {
			log.Printf("Failed to write output for %s: %v", host, err)
		}
	}

	return nil
}
