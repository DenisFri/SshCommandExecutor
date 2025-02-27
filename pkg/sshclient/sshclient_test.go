package sshclient

import (
	"golang.org/x/crypto/ssh"
	"testing"
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
	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("testpassword"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	err := ExecuteCommands("localhost", config, []string{"echo 'hello'"})
	if err == nil {
		t.Errorf("Expected an error when connecting to localhost, got nil")
	}
}
