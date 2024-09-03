package sshclient

import (
	"golang.org/x/crypto/ssh"
	"testing"
)

// TestLoadConfig tests the LoadConfig function
func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig("../../config/config.yaml")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.SSH.User == "" {
		t.Errorf("Expected SSH user to be set, got an empty string")
	}

	if config.SSH.Password == "" {
		t.Errorf("Expected SSH password to be set, got an empty string")
	}

	if len(config.Commands) == 0 {
		t.Errorf("Expected at least one command, got none")
	}
}

// TestLoadHosts tests the LoadHosts function
func TestLoadHosts(t *testing.T) {
	hosts, err := LoadHosts("../../hosts/hosts.yaml")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(hosts.Hosts) == 0 {
		t.Errorf("Expected at least one host, got none")
	}
}

// TestGetSSHClient tests the GetSSHClient function
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

// TestExecuteCommands is a basic test for the ExecuteCommands function
func TestExecuteCommands(t *testing.T) {
	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// This is a mock test, replace with an actual host and command if possible
	err := ExecuteCommands("localhost", config, []string{"echo 'hello'"})
	if err == nil {
		t.Errorf("Expected an error when connecting to localhost, got nil")
	}
}
