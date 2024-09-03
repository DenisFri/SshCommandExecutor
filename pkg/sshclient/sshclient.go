package sshclient

import (
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SSH struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"ssh"`
	Commands []string `yaml:"commands"`
}

// Hosts represents the list of target hosts
type Hosts struct {
	Hosts []string `yaml:"hosts"`
}

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

func GetSSHClient(user, password string) (*ssh.ClientConfig, error) {
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

		// Handle the error from session.Close()
		if err := session.Close(); err != nil {
			log.Printf("Failed to close session for %s: %v", host, err)
		}
	}

	return nil
}
