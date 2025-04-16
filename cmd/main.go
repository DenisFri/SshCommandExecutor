package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/DenisFri/SshCommandExecutor/pkg/sshclient"
)

func main() {
	// Parse command-line flags
	playbookPath := flag.String("playbooks", "config/playbooks.yaml", "Path to playbooks YAML file")
	hostsPath := flag.String("hosts", "config/hosts.yaml", "Path to hosts YAML file")
	outputDir := flag.String("output", "output", "Output directory for command results")
	concurrency := flag.Int("concurrency", 10, "Maximum number of concurrent SSH connections")
	timeout := flag.Duration("timeout", 5*time.Minute, "Global execution timeout")
	flag.Parse()

	// Create a cancellable context for the entire execution
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Setup signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		log.Println("Shutdown signal received, terminating...")
		cancel()
	}()

	// Load playbooks from playbooks.yaml
	playbookConfig, err := sshclient.LoadPlaybooks(*playbookPath)
	if err != nil {
		log.Fatalf("Error loading playbooks: %v", err)
	}

	// Load hosts from hosts.yaml
	hostsConfig, err := sshclient.LoadHosts(*hostsPath)
	if err != nil {
		log.Fatalf("Error loading hosts: %v", err)
	}

	// Create executor config
	config := sshclient.DefaultExecutorConfig()
	config.OutputDirectory = *outputDir
	config.ConcurrentLimit = *concurrency
	config.ExecuteTimeout = *timeout

	// Retrieve the SSH client configuration
	sshConfig, err := sshclient.GetSSHClient()
	if err != nil {
		log.Fatalf("Error configuring SSH client: %v", err)
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, config.ConcurrentLimit)
	var wg sync.WaitGroup

	// Execute the commands for each host, based on the assigned playbook
	for _, host := range hostsConfig.Hosts {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			log.Println("Execution cancelled, stopping...")
			break
		default:
			// Continue execution
		}

		wg.Add(1)
		// Acquire semaphore slot
		sem <- struct{}{}

		go func(h sshclient.HostConfig) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore when done

			// Find the playbook assigned to this host
			playbook, err := sshclient.FindPlaybook(playbookConfig, h.Playbook)
			if err != nil {
				log.Printf("Error finding playbook for host %s: %v", h.Hostname, err)
				return
			}

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
