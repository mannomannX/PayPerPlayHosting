package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

// AuthService interface for middleware
type AuthServiceInterface interface {
	ValidateToken(tokenString string) (*service.Claims, error)
}

var authService AuthServiceInterface

// SetAuthService sets the auth service for the middleware
func SetAuthService(svc AuthServiceInterface) {
	authService = svc
}

// AuthMiddleware validates JWT authentication tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization header",
				"code":  "UNAUTHORIZED",
			})
			c.Abort()
			return
		}

		// Extract token (format: "Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization format. Use: Bearer <token>",
				"code":  "INVALID_AUTH_FORMAT",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate JWT token
		if authService == nil {
			// Fallback for development/testing without auth
			c.Set("user_id", "default")
			c.Set("is_admin", true)
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("is_admin", claims.IsAdmin)

		c.Next()
	}
}

// OptionalAuthMiddleware allows requests with or without auth
// If auth is provided, it validates and sets user context
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth provided, continue without user context
			c.Next()
			return
		}

		// Extract token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, but don't abort
			c.Next()
			return
		}

		token := parts[1]

		// Validate token if auth service is available
		if authService != nil {
			claims, err := authService.ValidateToken(token)
			if err == nil {
				// Valid token, set context
				c.Set("user_id", claims.UserID)
				c.Set("email", claims.Email)
				c.Set("is_admin", claims.IsAdmin)
			}
		}

		c.Next()
	}
}

// RequireRole checks if user has required role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Not authenticated",
				"code":  "NOT_AUTHENTICATED",
			})
			c.Abort()
			return
		}

		if userRole != role && userRole != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
				"code":  "FORBIDDEN",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return "default"
	}
	return userID.(string)
}
