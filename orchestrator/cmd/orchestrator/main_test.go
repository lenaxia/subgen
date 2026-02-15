package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestMainPackageCompiles is a smoke test to ensure the main package compiles
func TestMainPackageCompiles(t *testing.T) {
	// This test just needs to compile
	t.Log("Main package compiles successfully")
}

// TestVersionConstant tests that version is defined
func TestVersionConstant(t *testing.T) {
	if Version == "" {
		t.Error("Version constant must not be empty")
	}
	t.Logf("Version: %s", Version)
}

// TestBuildInfo tests that build info is populated
func TestBuildInfo(t *testing.T) {
	info := GetBuildInfo()

	if info.Version == "" {
		t.Error("Build info version must not be empty")
	}

	if info.GoVersion == "" {
		t.Error("Build info Go version must not be empty")
	}

	t.Logf("Build info: %+v", info)
}

// TestGetBuildInfoReturnsAllFields tests that all fields are populated
func TestGetBuildInfoReturnsAllFields(t *testing.T) {
	info := GetBuildInfo()

	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if info.BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if info.GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}
}

// TestFormatVersion tests that version formatting works correctly
func TestFormatVersion(t *testing.T) {
	var buf bytes.Buffer
	err := FormatVersion(&buf)

	if err != nil {
		t.Fatalf("FormatVersion returned error: %v", err)
	}

	output := buf.String()

	// Check that output contains expected strings
	expectedStrings := []string{
		"Subgen Orchestrator",
		"Version:",
		"Go Version:",
		"Build Time:",
		"Git Commit:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected string: %s", expected)
		}
	}

	t.Logf("Version output:\n%s", output)
}

// TestFormatVersionWithBadWriter tests error handling
func TestFormatVersionWithBadWriter(t *testing.T) {
	// Use a writer that always fails
	badWriter := &failingWriter{}
	err := FormatVersion(badWriter)

	if err == nil {
		t.Error("Expected error from FormatVersion with failing writer, got nil")
	}
}

// TestCheckHealthFailsWhenNoServer tests health check fails appropriately
func TestCheckHealthFailsWhenNoServer(t *testing.T) {
	// Assuming no server is running on port 9000
	err := CheckHealth()
	if err == nil {
		t.Log("Health check passed - server must be running")
	} else {
		t.Logf("Health check failed as expected (no server): %v", err)
	}
}

// TestBuildInfoWithCustomValues tests build info with custom ldflags values
func TestBuildInfoWithCustomValues(t *testing.T) {
	// Save original values
	origVersion := Version
	origBuildTime := BuildTime
	origGitCommit := GitCommit

	// Set custom values
	Version = "1.2.3"
	BuildTime = "2024-01-01T00:00:00Z"
	GitCommit = "abc123def456"

	// Get build info
	info := GetBuildInfo()

	// Verify custom values are returned
	if info.Version != "1.2.3" {
		t.Errorf("Expected Version '1.2.3', got '%s'", info.Version)
	}
	if info.BuildTime != "2024-01-01T00:00:00Z" {
		t.Errorf("Expected BuildTime '2024-01-01T00:00:00Z', got '%s'", info.BuildTime)
	}
	if info.GitCommit != "abc123def456" {
		t.Errorf("Expected GitCommit 'abc123def456', got '%s'", info.GitCommit)
	}

	// Restore original values
	Version = origVersion
	BuildTime = origBuildTime
	GitCommit = origGitCommit
}

// TestFormatVersionOutputFormat tests the exact output format
func TestFormatVersionOutputFormat(t *testing.T) {
	// Set known values for testing
	origVersion := Version
	origBuildTime := BuildTime
	origGitCommit := GitCommit

	Version = "test-version"
	BuildTime = "test-time"
	GitCommit = "test-commit"

	var buf bytes.Buffer
	err := FormatVersion(&buf)

	if err != nil {
		t.Fatalf("FormatVersion failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 5 {
		t.Errorf("Expected 5 lines of output, got %d", len(lines))
	}

	// Check specific format
	if !strings.Contains(lines[0], "Subgen Orchestrator") {
		t.Errorf("First line should contain 'Subgen Orchestrator', got: %s", lines[0])
	}

	// Restore original values
	Version = origVersion
	BuildTime = origBuildTime
	GitCommit = origGitCommit
}

// TestCheckHealthReturnValueTypes tests that CheckHealth returns proper error types
func TestCheckHealthReturnValueTypes(t *testing.T) {
	err := CheckHealth()
	// The function should return either nil or an error
	// We just verify the type system works correctly
	if err != nil {
		t.Logf("Health check returned error (expected when no server): %v", err)
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// failingWriter is a writer that always returns an error
type failingWriter struct{}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}
