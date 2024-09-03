package sshclient

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

// Config represents the SSH and command configurations
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

// LoadConfig loads SSH and command configurations from a YAML file
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}
	data, err := ioutil.ReadFile(path)
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
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hosts file: %v", err)
	}
	if err := yaml.Unmarshal(data, hosts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hosts data: %v", err)
	}
	return hosts, nil
}

// Exported function with capitalized name
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

// ExecuteCommands connects to a host and executes the provided commands
func ExecuteCommands(host string, config *ssh.ClientConfig, commands []string) error {
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), config)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %v", host, err)
	}
	defer conn.Close()

	for _, cmd := range commands {
		session, err := conn.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %v", err)
		}
		defer session.Close()

		output, err := session.CombinedOutput(cmd)
		if err != nil {
			log.Printf("Failed to execute command '%s' on %s: %v", cmd, host, err)
			continue
		}

		log.Printf("Output from %s:\n%s", host, output)
	}

	return nil
}
