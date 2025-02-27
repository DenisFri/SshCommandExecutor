package sshclient

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

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

func GetSSHClient() (*ssh.ClientConfig, error) {
	decryptionPassword := os.Getenv("CREDENTIALS_PASSWORD")
	if decryptionPassword == "" {
		return nil, fmt.Errorf("CREDENTIALS_PASSWORD environment variable not set")
	}

	creds, err := DecryptCredentials(decryptionPassword, "config/credentials.enc")
	if err != nil {
		return nil, fmt.Errorf("error decrypting credentials: %v", err)
	}

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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	return config, nil
}

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
