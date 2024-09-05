package main

import (
	"log"
	"sync"
	"time"

	"github.com/DenisFri/SshCommandExecutor/pkg/sshclient"
)

func main() {
	// Load playbooks from playbooks.yaml
	playbookConfig, err := sshclient.LoadPlaybooks("config/playbooks.yaml")
	if err != nil {
		log.Fatalf("Error loading playbooks: %v", err)
	}

	// Load hosts from hosts.yaml
	hostsConfig, err := sshclient.LoadHosts("config/hosts.yaml")
	if err != nil {
		log.Fatalf("Error loading hosts: %v", err)
	}

	// Retrieve the SSH client configuration
	sshConfig, err := sshclient.GetSSHClient()
	if err != nil {
		log.Fatalf("Error configuring SSH client: %v", err)
	}

	// Execute the commands for each host, based on the assigned playbook
	var wg sync.WaitGroup
	for _, host := range hostsConfig.Hosts {
		wg.Add(1)
		go func(h sshclient.HostConfig) {
			defer wg.Done()

			// Find the playbook assigned to this host
			playbook, err := sshclient.FindPlaybook(playbookConfig, h.Playbook)
			if err != nil {
				log.Printf("Error finding playbook for host %s: %v", h.Hostname, err)
				return
			}

			// Introduce a delay between command executions for each host (optional)
			time.Sleep(500 * time.Millisecond)

			log.Printf("Connecting to %s with playbook %s...", h.Hostname, h.Playbook)
			if err := sshclient.ExecuteCommands(h.Hostname, sshConfig, playbook.Commands); err != nil {
				log.Printf("Error executing commands on %s: %v", h.Hostname, err)
			} else {
				log.Printf("Successfully executed commands on %s", h.Hostname)
			}
		}(host)
	}

	wg.Wait()
	log.Println("All commands executed.")
}
