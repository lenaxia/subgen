// Package middleware provides HTTP middleware for the orchestrator
package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// RequestID adds X-Request-ID to context and response headers for request tracing
func RequestID(log *logrus.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check for existing X-Request-ID header
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context
		c.Locals("request_id", requestID)

		// Set response header
		c.Set("X-Request-ID", requestID)

		// Add to logger context if available
		if log != nil {
			c.Locals("logger", log.WithField("request_id", requestID))
		}

		return c.Next()
	}
}

// GetRequestID retrieves request ID from context
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals("request_id").(string); ok {
		return id
	}
	return ""
}

// GetLogger retrieves logger with request ID from context
func GetLogger(c *fiber.Ctx, fallback *logrus.Logger) *logrus.Entry {
	if logger, ok := c.Locals("logger").(*logrus.Entry); ok {
		return logger
	}
	if fallback != nil {
		requestID := GetRequestID(c)
		if requestID != "" {
			return fallback.WithField("request_id", requestID)
		}
		return logrus.NewEntry(fallback)
	}
	return logrus.NewEntry(logrus.StandardLogger())
}
