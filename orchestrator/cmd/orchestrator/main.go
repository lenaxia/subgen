package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Version is the orchestrator version (set at build time)
var Version = "dev"

// BuildTime is the build timestamp (set at build time via ldflags)
var BuildTime = "unknown"

// GitCommit is the git commit hash (set at build time via ldflags)
var GitCommit = "unknown"

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	GoVersion string
	BuildTime string
	GitCommit string
}

// GetBuildInfo returns build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GoVersion: runtime.Version(),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}
}

func main() {
	// Check for --health flag (Docker health check)
	if len(os.Args) > 1 && os.Args[1] == "--health" {
		if err := CheckHealth(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
		os.Exit(0)
	}

	// Check for --version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		PrintVersion()
		return
	}

	// Setup structured logging
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetLevel(logrus.InfoLevel)

	// Log startup
	buildInfo := GetBuildInfo()
	log.WithFields(logrus.Fields{
		"version":    buildInfo.Version,
		"go_version": buildInfo.GoVersion,
		"build_time": buildInfo.BuildTime,
		"git_commit": buildInfo.GitCommit,
	}).Info("Subgen Orchestrator starting")

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.WithField("signal", sig).Info("Received shutdown signal")
		cancel()
	}()

	// FUTURE: Initialize components (config, queue, HTTP server, gRPC client)
	// This will be implemented in subsequent stories

	log.Info("Orchestrator initialized successfully")

	// Wait for shutdown signal
	<-ctx.Done()

	log.Info("Shutting down gracefully")

	// FUTURE: Cleanup resources
	// This will be implemented in subsequent stories

	log.Info("Shutdown complete")
}

// FormatVersion formats version information to the writer
func FormatVersion(w io.Writer) error {
	info := GetBuildInfo()
	_, err := fmt.Fprintf(w, "Subgen Orchestrator\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Version:    %s\n", info.Version)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Go Version: %s\n", info.GoVersion)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Build Time: %s\n", info.BuildTime)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Git Commit: %s\n", info.GitCommit)
	return err
}

// PrintVersion prints the version information and exits
func PrintVersion() {
	_ = FormatVersion(os.Stdout)
	os.Exit(0)
}

// CheckHealth performs a health check on the orchestrator
// Currently checks if the HTTP port (9000) is available/listening
func CheckHealth() error {
	// FUTURE: Add more comprehensive health checks (database, queue, etc.)

	// Check if we can connect to the HTTP port
	// For now, just verify the service can bind to the port or is already listening
	conn, err := net.DialTimeout("tcp", "localhost:9000", 2*time.Second)
	if err != nil {
		// If we can't connect, the service might not be up yet
		return fmt.Errorf("HTTP server not responding on port 9000: %w", err)
	}
	conn.Close()
	return nil
}
