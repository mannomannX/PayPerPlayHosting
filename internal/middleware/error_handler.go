package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/pkg/logger"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ErrorHandler is a middleware that catches panics and errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered", err.(error), map[string]interface{}{
					"path":   c.Request.URL.Path,
					"method": c.Request.Method,
				})

				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Internal server error",
					Message: "An unexpected error occurred",
					Code:    "INTERNAL_ERROR",
				})

				c.Abort()
			}
		}()

		c.Next()

		// Check if there were any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			logger.Error("Request error", err.Err, map[string]interface{}{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
			})

			// If response not already written
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   err.Error(),
					Message: "Request failed",
				})
			}
		}
	}
}

// Custom error types for better error handling

type AppError struct {
	StatusCode int
	Code       string
	Message    string
	Err        error
	Details    map[string]interface{}
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func NewBadRequestError(message string) *AppError {
	return &AppError{
		StatusCode: http.StatusBadRequest,
		Code:       "BAD_REQUEST",
		Message:    message,
	}
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Message:    resource + " not found",
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		StatusCode: http.StatusInternalServerError,
		Code:       "INTERNAL_ERROR",
		Message:    "Internal server error",
		Err:        err,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		StatusCode: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    message,
	}
}

// HandleAppError handles AppError types
func HandleAppError(c *gin.Context, err *AppError) {
	logger.Error(err.Message, err.Err, map[string]interface{}{
		"code":   err.Code,
		"status": err.StatusCode,
		"path":   c.Request.URL.Path,
	})

	response := ErrorResponse{
		Error:   err.Message,
		Code:    err.Code,
		Details: err.Details,
	}

	c.JSON(err.StatusCode, response)
	c.Abort()
}
