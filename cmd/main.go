package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/DenisFri/SshCommandExecutor/pkg/sshclient"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Parse command-line flags
	playbookPath := flag.String("playbooks", "config/playbooks.yaml", "Path to playbooks YAML file")
	hostsPath := flag.String("hosts", "config/hosts.yaml", "Path to hosts YAML file")
	credentialsPath := flag.String("credentials", "config/credentials.enc", "Path to encrypted credentials file")
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
		slog.Info("Shutdown signal received, terminating...")
		cancel()
	}()

	// Load playbooks from playbooks.yaml
	playbookConfig, err := sshclient.LoadPlaybooks(*playbookPath)
	if err != nil {
		slog.Error("Error loading playbooks", "error", err, "path", *playbookPath)
		os.Exit(1)
	}

	// Load hosts from hosts.yaml
	hostsConfig, err := sshclient.LoadHosts(*hostsPath)
	if err != nil {
		slog.Error("Error loading hosts", "error", err, "path", *hostsPath)
		os.Exit(1)
	}

	// Create executor config
	config := sshclient.DefaultExecutorConfig()
	config.OutputDirectory = *outputDir
	config.ConcurrentLimit = *concurrency
	config.ExecuteTimeout = *timeout
	config.CredentialsPath = *credentialsPath

	// Validate configuration
	if err := config.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Retrieve the SSH client configuration
	sshConfig, err := sshclient.GetSSHClient(config.CredentialsPath)
	if err != nil {
		slog.Error("Error configuring SSH client", "error", err)
		os.Exit(1)
	}

	// Create a semaphore to limit concurrency
	sem := make(chan struct{}, config.ConcurrentLimit)
	var wg sync.WaitGroup

	// Create a channel to collect execution results
	type ExecutionResult struct {
		Host           string
		Error          error
		CommandsCount  int
		SuccessCount   int
		FailedCount    int
	}
	results := make(chan ExecutionResult, len(hostsConfig.Hosts))

	// Progress tracking with atomic counters
	var (
		totalHosts       = int64(len(hostsConfig.Hosts))
		completedHosts   atomic.Int64
		succeededHosts   atomic.Int64
		failedHostsCount atomic.Int64
		totalCommands    atomic.Int64
		completedCmds    atomic.Int64
		succeededCmds    atomic.Int64
		failedCmds       atomic.Int64
	)

	// Calculate total commands
	for _, host := range hostsConfig.Hosts {
		playbook, err := sshclient.FindPlaybook(playbookConfig, host.Playbook)
		if err == nil {
			totalCommands.Add(int64(len(playbook.Commands)))
		}
	}

	// Start progress reporting goroutine
	progressCtx, progressCancel := context.WithCancel(ctx)
	defer progressCancel()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				completed := completedHosts.Load()
				total := totalHosts
				succeededH := succeededHosts.Load()
				failedH := failedHostsCount.Load()
				completedC := completedCmds.Load()
				totalC := totalCommands.Load()
				succeededC := succeededCmds.Load()
				failedC := failedCmds.Load()

				// Calculate progress percentage
				var hostProgress, cmdProgress float64
				if total > 0 {
					hostProgress = float64(completed) / float64(total) * 100
				}
				if totalC > 0 {
					cmdProgress = float64(completedC) / float64(totalC) * 100
				}

				slog.Info("Progress update",
					"hosts_completed", completed,
					"hosts_total", total,
					"hosts_progress", fmt.Sprintf("%.1f%%", hostProgress),
					"hosts_succeeded", succeededH,
					"hosts_failed", failedH,
					"commands_completed", completedC,
					"commands_total", totalC,
					"commands_progress", fmt.Sprintf("%.1f%%", cmdProgress),
					"commands_succeeded", succeededC,
					"commands_failed", failedC)
			case <-progressCtx.Done():
				return
			}
		}
	}()

	// Execute the commands for each host, based on the assigned playbook
	for _, host := range hostsConfig.Hosts {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			slog.Info("Execution cancelled, stopping...")
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

			var execErr error
			var cmdCount, successCount, failCount int

			playbook, err := sshclient.FindPlaybook(playbookConfig, h.Playbook)
			if err != nil {
				slog.Error("Error finding playbook for host", "host", h.Hostname, "playbook", h.Playbook, "error", err)
				completedHosts.Add(1)
				failedHostsCount.Add(1)
				results <- ExecutionResult{Host: h.Hostname, Error: err, CommandsCount: 0, SuccessCount: 0, FailedCount: 0}
				return
			}

			cmdCount = len(playbook.Commands)

			// Create progress tracker for this host
			tracker := &sshclient.ProgressTracker{
				CompletedCommands: &completedCmds,
				SucceededCommands: &succeededCmds,
				FailedCommands:    &failedCmds,
			}

			slog.Info("Connecting to host", "host", h.Hostname, "playbook", h.Playbook)
			if err := sshclient.ExecuteCommands(ctx, h.Hostname, sshConfig, playbook.Commands, tracker); err != nil {
				slog.Error("Error executing commands", "host", h.Hostname, "error", err)
				execErr = err
				failedHostsCount.Add(1)
			} else {
				slog.Info("Successfully executed commands", "host", h.Hostname)
				succeededHosts.Add(1)
			}

			// Mark host as completed
			completedHosts.Add(1)

			results <- ExecutionResult{
				Host:          h.Hostname,
				Error:         execErr,
				CommandsCount: cmdCount,
				SuccessCount:  successCount,
				FailedCount:   failCount,
			}
		}(host)
	}

	wg.Wait()
	close(results)

	// Stop progress reporting goroutine
	progressCancel()

	// Process results to collect failed host names
	var failedHostsList []string
	for result := range results {
		if result.Error != nil {
			failedHostsList = append(failedHostsList, result.Host)
		}
	}

	// Get final counts from atomic counters
	finalCompletedHosts := completedHosts.Load()
	finalSucceededHosts := succeededHosts.Load()
	finalFailedHosts := failedHostsCount.Load()
	finalCompletedCmds := completedCmds.Load()
	finalSucceededCmds := succeededCmds.Load()
	finalFailedCmds := failedCmds.Load()

	// Log detailed execution summary
	slog.Info("=== Execution Summary ===")
	slog.Info("Host Statistics",
		"total_hosts", totalHosts,
		"completed_hosts", finalCompletedHosts,
		"succeeded_hosts", finalSucceededHosts,
		"failed_hosts", finalFailedHosts)

	slog.Info("Command Statistics",
		"total_commands", totalCommands.Load(),
		"completed_commands", finalCompletedCmds,
		"succeeded_commands", finalSucceededCmds,
		"failed_commands", finalFailedCmds)

	if finalFailedHosts > 0 {
		slog.Warn("Some hosts failed", "failed_hosts", failedHostsList)
		os.Exit(1)
	}

	slog.Info("All hosts completed successfully!")
}
