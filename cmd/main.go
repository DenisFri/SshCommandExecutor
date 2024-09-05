package main

import (
	"log"
	"sync"
	"time"

	"github.com/DenisFri/SshCommandExecutor/pkg/sshclient"
)

func main() {
	// Load configuration and commands from config.yaml
	config, err := sshclient.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Load the list of hosts from hosts.yaml
	hosts, err := sshclient.LoadHosts("hosts/hosts.yaml")
	if err != nil {
		log.Fatalf("Error loading hosts: %v", err)
	}

	// Retrieve the SSH client configuration
	sshConfig, err := sshclient.GetSSHClient()
	if err != nil {
		log.Fatalf("Error configuring SSH client: %v", err)
	}

	// Execute the commands concurrently across the hosts with a delay
	var wg sync.WaitGroup
	for _, host := range hosts.Hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()

			time.Sleep(1000 * time.Millisecond)

			log.Printf("Connecting to %s...", h)
			if err := sshclient.ExecuteCommands(h, sshConfig, config.Commands); err != nil {
				log.Printf("Error executing commands on %s: %v", h, err)
			} else {
				log.Printf("Successfully executed commands on %s", h)
			}
		}(host)
	}

	wg.Wait()
	log.Println("All commands executed.")
}
