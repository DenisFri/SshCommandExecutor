package sshclient

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v2"
)

// ProgressTracker tracks execution progress using atomic counters
type ProgressTracker struct {
	CompletedCommands *atomic.Int64
	SucceededCommands *atomic.Int64
	FailedCommands    *atomic.Int64
}

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

	// Credentials settings
	CredentialsPath string

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
		CredentialsPath: "config/credentials.enc",
		OutputDirectory: "output",
		ConcurrentLimit: 10,
	}
}

// Validate checks the configuration for invalid values
func (c *ExecutorConfig) Validate() error {
	if c.ConcurrentLimit <= 0 {
		return fmt.Errorf("concurrent limit must be positive, got %d", c.ConcurrentLimit)
	}
	if c.ConcurrentLimit > 1000 {
		return fmt.Errorf("concurrent limit too high (max 1000), got %d", c.ConcurrentLimit)
	}
	if c.ConnectTimeout <= 0 {
		return fmt.Errorf("connect timeout must be positive, got %v", c.ConnectTimeout)
	}
	if c.ExecuteTimeout <= 0 {
		return fmt.Errorf("execute timeout must be positive, got %v", c.ExecuteTimeout)
	}
	if c.KeepAliveTime < 0 {
		return fmt.Errorf("keep alive time cannot be negative, got %v", c.KeepAliveTime)
	}
	if c.DefaultPort <= 0 || c.DefaultPort > 65535 {
		return fmt.Errorf("default port must be between 1 and 65535, got %d", c.DefaultPort)
	}
	if c.CredentialsPath == "" {
		return fmt.Errorf("credentials path cannot be empty")
	}
	if c.OutputDirectory == "" {
		return fmt.Errorf("output directory cannot be empty")
	}
	return nil
}

// Playbook defines a collection of reusable command sequences
type Playbook struct {
	Name     string   `yaml:"name"`
	Commands []string `yaml:"commands"`
}

type PlaybookConfig struct {
	Playbooks []Playbook `yaml:"playbooks"`
}

type HostConfig struct {
	Hostname string `yaml:"hostname"`
	Playbook string `yaml:"playbook"`
}

type HostsConfig struct {
	Hosts []HostConfig `yaml:"hosts"`
}

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

func FindPlaybook(playbookConfig *PlaybookConfig, playbookName string) (*Playbook, error) {
	for _, playbook := range playbookConfig.Playbooks {
		if playbook.Name == playbookName {
			return &playbook, nil
		}
	}
	return nil, fmt.Errorf("playbook %s not found", playbookName)
}

func GetSSHClient(credentialsPath string) (*ssh.ClientConfig, error) {
	decryptionPassword := os.Getenv("CREDENTIALS_PASSWORD")
	if decryptionPassword == "" {
		return nil, fmt.Errorf("CREDENTIALS_PASSWORD environment variable not set")
	}

	creds, err := DecryptCredentials(decryptionPassword, credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("error decrypting credentials: %v", err)
	}

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

// executeWithRetry executes a function with exponential backoff retry logic
func executeWithRetry(ctx context.Context, maxRetries int, operation func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check if context is cancelled before attempting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		if err := operation(); err != nil {
			lastErr = err

			// Don't retry if this was the last attempt
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
			}

			// Calculate exponential backoff: 2^attempt seconds (max 30 seconds)
			backoffSeconds := math.Pow(2, float64(attempt))
			if backoffSeconds > 30 {
				backoffSeconds = 30
			}
			waitDuration := time.Duration(backoffSeconds) * time.Second

			log.Printf("Attempt %d failed, retrying in %v: %v", attempt+1, waitDuration, err)

			// Wait with context awareness
			select {
			case <-time.After(waitDuration):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Success
		return nil
	}
	return lastErr
}

func ExecuteCommands(ctx context.Context, host string, config *ssh.ClientConfig, commands []string, tracker *ProgressTracker) error {
	var conn *ssh.Client

	// Connect with retry logic (max 3 attempts)
	err := executeWithRetry(ctx, 3, func() error {
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

		conn = ssh.NewClient(clientConn, chans, reqs)
		return nil
	})

	if err != nil {
		return err
	}
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

		// Close the session immediately after command execution
		session.Close()

		// Combine output
		output := outputBuf.String()
		errorOutput := errorBuf.String()

		if errorOutput != "" {
			output += "\nStderr: " + errorOutput
		}

		if execErr != nil {
			log.Printf("Failed to execute command '%s' on %s: %v", cmd, host, execErr)
			// Increment failed command counter if tracker is provided
			if tracker != nil && tracker.FailedCommands != nil {
				tracker.FailedCommands.Add(1)
			}
		} else {
			// Increment succeeded command counter if tracker is provided
			if tracker != nil && tracker.SucceededCommands != nil {
				tracker.SucceededCommands.Add(1)
			}
		}

		// Increment completed command counter if tracker is provided
		if tracker != nil && tracker.CompletedCommands != nil {
			tracker.CompletedCommands.Add(1)
		}

		// Write the output to file
		if _, err := outputFile.WriteString(fmt.Sprintf("Command: %s\nOutput:\n%s\n---\n", cmd, output)); err != nil {
			log.Printf("Failed to write output for %s: %v", host, err)
		}
	}

	return nil
}
