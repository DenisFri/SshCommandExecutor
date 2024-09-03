package main

import (
	"log"
	"sync"

	"github.com/yourusername/ssh-command-executor/pkg/sshclient"
)

func main() {
	config, err := sshclient.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	hosts, err := sshclient.LoadHosts("hosts/hosts.yaml")
	if err != nil {
		log.Fatalf("Error loading hosts: %v", err)
	}

	sshConfig, err := sshclient.getSSHClient(config.SSH.User, config.SSH.Password)
	if err != nil {
		log.Fatalf("Error configuring SSH client: %v", err)
	}

	var wg sync.WaitGroup
	for _, host := range hosts.Hosts {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
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
