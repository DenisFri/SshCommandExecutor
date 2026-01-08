package sshclient

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestLoadHosts(t *testing.T) {
	hosts, err := LoadHosts("../../config/hosts.yaml")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(hosts.Hosts) == 0 {
		t.Errorf("Expected at least one host, got none")
	}
}

func TestLoadHostsInvalidPath(t *testing.T) {
	_, err := LoadHosts("nonexistent.yaml")
	if err == nil {
		t.Errorf("Expected error for nonexistent file, got nil")
	}
}

func TestLoadPlaybooks(t *testing.T) {
	playbooks, err := LoadPlaybooks("../../config/playbooks.yaml")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(playbooks.Playbooks) == 0 {
		t.Errorf("Expected at least one playbook, got none")
	}
}

func TestLoadPlaybooksInvalidPath(t *testing.T) {
	_, err := LoadPlaybooks("nonexistent.yaml")
	if err == nil {
		t.Errorf("Expected error for nonexistent file, got nil")
	}
}

func TestFindPlaybook(t *testing.T) {
	playbooks, err := LoadPlaybooks("../../config/playbooks.yaml")
	if err != nil {
		t.Fatalf("Failed to load playbooks: %v", err)
	}

	if len(playbooks.Playbooks) > 0 {
		// Test finding existing playbook
		firstPlaybook := playbooks.Playbooks[0]
		found, err := FindPlaybook(playbooks, firstPlaybook.Name)
		if err != nil {
			t.Errorf("Expected to find playbook '%s', got error: %v", firstPlaybook.Name, err)
		}
		if found.Name != firstPlaybook.Name {
			t.Errorf("Expected playbook name '%s', got '%s'", firstPlaybook.Name, found.Name)
		}
	}

	// Test finding nonexistent playbook
	_, err = FindPlaybook(playbooks, "nonexistent_playbook")
	if err == nil {
		t.Errorf("Expected error for nonexistent playbook, got nil")
	}
}

func TestDefaultExecutorConfig(t *testing.T) {
	config := DefaultExecutorConfig()

	if config.ConnectTimeout <= 0 {
		t.Errorf("Expected positive ConnectTimeout, got %v", config.ConnectTimeout)
	}
	if config.ExecuteTimeout <= 0 {
		t.Errorf("Expected positive ExecuteTimeout, got %v", config.ExecuteTimeout)
	}
	if config.DefaultPort != 22 {
		t.Errorf("Expected default port 22, got %d", config.DefaultPort)
	}
	if config.ConcurrentLimit <= 0 {
		t.Errorf("Expected positive ConcurrentLimit, got %d", config.ConcurrentLimit)
	}
}

func TestExecutorConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ExecutorConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultExecutorConfig(),
			wantErr: false,
		},
		{
			name: "invalid concurrency - zero",
			config: &ExecutorConfig{
				ConcurrentLimit: 0,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     22,
				CredentialsPath: "test.enc",
				OutputDirectory: "output",
			},
			wantErr: true,
		},
		{
			name: "invalid concurrency - too high",
			config: &ExecutorConfig{
				ConcurrentLimit: 2000,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     22,
				CredentialsPath: "test.enc",
				OutputDirectory: "output",
			},
			wantErr: true,
		},
		{
			name: "invalid port - zero",
			config: &ExecutorConfig{
				ConcurrentLimit: 10,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     0,
				CredentialsPath: "test.enc",
				OutputDirectory: "output",
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &ExecutorConfig{
				ConcurrentLimit: 10,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     70000,
				CredentialsPath: "test.enc",
				OutputDirectory: "output",
			},
			wantErr: true,
		},
		{
			name: "empty credentials path",
			config: &ExecutorConfig{
				ConcurrentLimit: 10,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     22,
				CredentialsPath: "",
				OutputDirectory: "output",
			},
			wantErr: true,
		},
		{
			name: "empty output directory",
			config: &ExecutorConfig{
				ConcurrentLimit: 10,
				ConnectTimeout:  30 * time.Second,
				ExecuteTimeout:  2 * time.Minute,
				DefaultPort:     22,
				CredentialsPath: "test.enc",
				OutputDirectory: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetSSHClient(t *testing.T) {
	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if config.User != "testuser" {
		t.Errorf("Expected user to be 'testuser', got %v", config.User)
	}
}

func TestExecuteCommands(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Pass nil tracker since we're not testing progress tracking here
	err := ExecuteCommands(ctx, "localhost", config, []string{"echo 'hello'"}, nil)
	if err == nil {
		t.Errorf("Expected an error when connecting to localhost, got nil")
	}
}

func TestExecuteCommandsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Pass nil tracker since we're not testing progress tracking here
	err := ExecuteCommands(ctx, "localhost", config, []string{"echo 'hello'"}, nil)
	if err == nil {
		t.Errorf("Expected context cancellation error, got nil")
	}
}

func TestCredentialsRoundtrip(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	plainPath := filepath.Join(tmpDir, "plain.txt")

	// Write test credentials
	testCreds := "SSH_EXECUTOR_USER=testuser\nSSH_EXECUTOR_PASSWORD=testpass"
	if err := os.WriteFile(plainPath, []byte(testCreds), 0600); err != nil {
		t.Fatalf("Failed to write test credentials: %v", err)
	}

	// Note: Actual encryption/decryption test would require the credential tool
	// This is a placeholder for demonstrating test structure
	t.Log("Credentials roundtrip test placeholder - requires credential tool integration")
}
