package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesNewID(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetOutput(io.Discard)

	app.Use(RequestID(log))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify
	requestID := resp.Header.Get("X-Request-ID")
	assert.NotEmpty(t, requestID, "X-Request-ID header should be set")
	assert.Len(t, requestID, 36, "Should be a valid UUID (36 characters)")
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetOutput(io.Discard)

	existingID := "test-request-id-123"

	app.Use(RequestID(log))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify
	requestID := resp.Header.Get("X-Request-ID")
	assert.Equal(t, existingID, requestID, "Should preserve existing X-Request-ID")
}

func TestRequestID_StoresInContext(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetOutput(io.Discard)

	var contextRequestID string

	app.Use(RequestID(log))
	app.Get("/test", func(c *fiber.Ctx) error {
		contextRequestID = GetRequestID(c)
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify
	headerRequestID := resp.Header.Get("X-Request-ID")
	assert.Equal(t, headerRequestID, contextRequestID, "Context ID should match header ID")
}

func TestGetRequestID_ReturnsEmptyWhenNotSet(t *testing.T) {
	// Setup
	app := fiber.New()
	var retrievedID string

	app.Get("/test", func(c *fiber.Ctx) error {
		retrievedID = GetRequestID(c)
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)

	// Verify
	assert.Empty(t, retrievedID, "Should return empty string when not set")
}

func TestGetLogger_ReturnsLoggerWithRequestID(t *testing.T) {
	// Setup
	app := fiber.New()
	log := logrus.New()
	log.SetOutput(io.Discard)

	app.Use(RequestID(log))
	app.Get("/test", func(c *fiber.Ctx) error {
		logger := GetLogger(c, log)

		// Verify logger has request_id field
		entry := logger.WithField("test", "value")
		assert.Contains(t, entry.Data, "request_id")
		assert.NotEmpty(t, entry.Data["request_id"])

		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
}

func TestGetLogger_ReturnsFallbackWhenNoLogger(t *testing.T) {
	// Setup
	app := fiber.New()
	fallback := logrus.New()
	fallback.SetOutput(io.Discard)

	app.Get("/test", func(c *fiber.Ctx) error {
		logger := GetLogger(c, fallback)
		assert.NotNil(t, logger, "Should return fallback logger")
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
}

func TestRequestID_NilLoggerHandled(t *testing.T) {
	// Setup
	app := fiber.New()

	// Should not panic with nil logger
	app.Use(RequestID(nil))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Execute
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify - should still set request ID even without logger
	requestID := resp.Header.Get("X-Request-ID")
	assert.NotEmpty(t, requestID)
}
