package webhooks

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// handleQueueStatus returns current queue statistics
// GET /queue/status
func (s *Server) handleQueueStatus() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get queue metrics
		queued := s.queue.Size()
		processing := s.queue.ProcessingCount()
		idle := s.queue.IsIdle()

		status := "active"
		if idle {
			status = "idle"
		}

		// TODO: Add completed/failed counts from last hour
		// This requires tracking completion times in history

		return c.JSON(fiber.Map{
			"status":     status,
			"queued":     queued,
			"processing": processing,
			"idle":       idle,
			"workers": fiber.Map{
				"total":  2, // TODO: Get from config
				"active": processing,
				"idle":   2 - processing,
			},
		})
	}
}

// handleQueueProcessing returns list of currently processing tasks
// GET /queue/processing
func (s *Server) handleQueueProcessing() fiber.Handler {
	return func(c *fiber.Ctx) error {
		processingTasks := s.queue.GetAllProcessingTaskInfo()

		return c.JSON(fiber.Map{
			"tasks": processingTasks,
		})
	}
}

// handleQueueHistory returns recent task completions with pagination
// GET /queue/history?limit=100&offset=0
func (s *Server) handleQueueHistory() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse pagination parameters
		limit := c.QueryInt("limit", 100)
		offset := c.QueryInt("offset", 0)

		// Get history
		tasks := s.queue.GetHistory(limit, offset)
		total := s.queue.GetHistoryTotal()

		return c.JSON(fiber.Map{
			"tasks":  tasks,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// handleTaskStatus returns detailed status for a specific task
// GET /tasks/:id
func (s *Server) handleTaskStatus() fiber.Handler {
	return func(c *fiber.Ctx) error {
		taskID := c.Params("id")

		taskInfo := s.queue.GetTaskInfo(taskID)
		if taskInfo == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "error",
				"error":  "task not found: " + taskID,
			})
		}

		return c.JSON(taskInfo)
	}
}

// RegisterQueueStatusRoutes registers the queue status and monitoring routes
func (s *Server) RegisterQueueStatusRoutes() {
	s.app.Get("/queue/status", s.handleQueueStatus())
	s.app.Get("/queue/processing", s.handleQueueProcessing())
	s.app.Get("/queue/history", s.handleQueueHistory())
	s.app.Get("/tasks/:id", s.handleTaskStatus())
}

// QueueStats contains uptime and timing information
type QueueStats struct {
	UptimeSeconds int64     `json:"uptime_seconds"`
	StartTime     time.Time `json:"start_time"`
}

// startTime tracks when the server started (for uptime calculation)
var startTime = time.Now()

// GetUptime returns server uptime in seconds
func GetUptime() int64 {
	return int64(time.Since(startTime).Seconds())
}
