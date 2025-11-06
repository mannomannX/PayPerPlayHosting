package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/pkg/logger"
)

// RequestLogger logs all HTTP requests with structured logging
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Log request
		fields := map[string]interface{}{
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"status":     c.Writer.Status(),
			"latency_ms": latency.Milliseconds(),
			"ip":         c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
		}

		// Add user ID if authenticated
		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}

		// Log based on status code
		status := c.Writer.Status()
		message := "HTTP request"

		if status >= 500 {
			logger.Error(message, nil, fields)
		} else if status >= 400 {
			logger.Warn(message, fields)
		} else {
			logger.Info(message, fields)
		}
	}
}
