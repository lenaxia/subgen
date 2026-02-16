package webhooks

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
)

// Ensure monitor package is imported
var _ monitor.Scanner = nil

// handleBatch processes batch directory scanning requests
func (s *Server) handleBatch(c *fiber.Ctx) error {
	// Check if scanner is available
	if s.scanner == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "error",
			"error":  "batch processing not available (scanner not initialized)",
		})
	}

	// Get query parameters
	directory := c.Query("directory")
	recursive := c.QueryBool("recursive", false)
	language := c.Query("language", "")

	// Validate required parameters
	if directory == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  "directory parameter required",
		})
	}

	s.log.WithFields(map[string]interface{}{
		"directory": directory,
		"recursive": recursive,
		"language":  language,
	}).Info("Batch scan request received")

	// Validate directory exists
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  fmt.Sprintf("directory not found: %s", directory),
			})
		}
		if os.IsPermission(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "error",
				"error":  fmt.Sprintf("permission denied: %s", directory),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  fmt.Sprintf("failed to access directory: %v", err),
		})
	}

	// Verify it's a directory
	if !info.IsDir() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  fmt.Sprintf("path is not a directory: %s", directory),
		})
	}

	// Use scanner to scan directory
	result, err := s.scanner.ScanDirectory(directory, recursive, language)
	if err != nil {
		s.log.WithError(err).Error("Failed to scan directory")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error",
			"error":  err.Error(),
		})
	}

	s.log.WithFields(map[string]interface{}{
		"scanned": result.Scanned,
		"queued":  result.Queued,
		"skipped": result.Skipped,
	}).Info("Batch scan completed")

	// Return results
	return c.JSON(fiber.Map{
		"status":       "success",
		"scanned":      result.Scanned,
		"queued":       result.Queued,
		"skipped":      result.Skipped,
		"skip_reasons": result.SkipReasons,
	})
}
