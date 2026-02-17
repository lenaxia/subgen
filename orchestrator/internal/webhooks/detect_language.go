package webhooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

const (
	detectLanguageTimeout = 30 * time.Second
	maxUploadSize         = 500 * 1024 * 1024 // 500MB
)

// DetectLanguageResponse represents the JSON response
type DetectLanguageResponse struct {
	Language   string  `json:"language"`
	Code       string  `json:"code"`
	Confidence float64 `json:"confidence"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

// handleDetectLanguage handles POST /detect-language
// This endpoint accepts an uploaded audio file and returns detected language information.
// Query Parameters:
//   - offset (optional, default: 0): Start offset in seconds
//   - length (optional, default: 30): Length in seconds to analyze
//
// Returns: JSON with language name, code, and confidence
func (s *Server) handleDetectLanguage(c *fiber.Ctx) error {
	// Parse query parameters
	offsetStr := c.Query("offset", "0")
	lengthStr := c.Query("length", "30")

	offset, err := strconv.ParseFloat(offsetStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("invalid offset parameter: %v", err),
		})
	}

	length, err := strconv.ParseFloat(lengthStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("invalid length parameter: %v", err),
		})
	}

	// Validate parameters
	if offset < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "offset must be >= 0",
		})
	}
	if length <= 0 || length > 300 {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "length must be between 0 and 300 seconds",
		})
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  "no file uploaded or invalid multipart form",
		})
	}

	// Check file size
	if file.Size > maxUploadSize {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("file too large: %d bytes (max: %d)", file.Size, maxUploadSize),
		})
	}

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to open uploaded file: %v", err),
		})
	}
	defer src.Close()

	// Create temporary file in shared media volume accessible by worker
	tmpFile, err := os.CreateTemp("/media", "detect-*.tmp")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to create temp file: %v", err),
		})
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up

	// Copy uploaded file to temp file
	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to save uploaded file: %v", err),
		})
	}
	tmpFile.Close()

	// Set world-readable permissions so worker container can access file
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("failed to set file permissions: %v", err),
		})
	}

	// Select a worker
	if s.workerPool == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Status: "error",
			Error:  "worker pool not initialized",
		})
	}

	worker, err := s.workerPool.SelectWorker()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("no workers available: %v", err),
		})
	}

	// Call worker's DetectLanguage RPC
	if s.grpcClient == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  "gRPC client not initialized",
		})
	}

	ctx, cancel := context.WithTimeout(c.Context(), detectLanguageTimeout)
	defer cancel()

	s.log.WithFields(logrus.Fields{
		"file_size":   file.Size,
		"offset":      offset,
		"length":      length,
		"worker_addr": worker.Address,
	}).Info("Detecting language")

	resp, err := s.grpcClient.DetectLanguage(ctx, worker.Address, tmpPath, offset, length)
	if err != nil {
		s.log.WithError(err).Error("Language detection failed")
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Status: "error",
			Error:  fmt.Sprintf("language detection failed: %v", err),
		})
	}

	s.log.WithFields(logrus.Fields{
		"language":   resp.LanguageName,
		"code":       resp.LanguageCode,
		"confidence": resp.Confidence,
	}).Info("Language detected")

	// Return response
	return c.JSON(DetectLanguageResponse{
		Language:   resp.LanguageName,
		Code:       resp.LanguageCode,
		Confidence: float64(resp.Confidence),
	})
}
