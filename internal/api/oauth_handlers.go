package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
)

// OAuthHandler handles OAuth authentication endpoints
type OAuthHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
	}
}

// DiscordLogin initiates Discord OAuth flow
// GET /api/auth/oauth/discord
func (h *OAuthHandler) DiscordLogin(c *gin.Context) {
	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	authURL, err := h.oauthService.GenerateAuthURL(models.OAuthProviderDiscord, userAgent, ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authorization URL",
		})
		return
	}

	// Return the auth URL for frontend to redirect
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"provider": "discord",
	})
}

// DiscordCallback handles Discord OAuth callback
// GET /api/auth/oauth/discord/callback
func (h *OAuthHandler) DiscordCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing code or state parameter",
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	token, user, isNewDevice, err := h.oauthService.HandleCallback(
		models.OAuthProviderDiscord,
		code,
		state,
		userAgent,
		ipAddress,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "OAuth authentication failed",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
			"balance":  user.Balance,
			"is_admin": user.IsAdmin,
		},
		"token":         token,
		"is_new_device": isNewDevice,
		"provider":      "discord",
	})
}

// GoogleLogin initiates Google OAuth flow
// GET /api/auth/oauth/google
func (h *OAuthHandler) GoogleLogin(c *gin.Context) {
	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	authURL, err := h.oauthService.GenerateAuthURL(models.OAuthProviderGoogle, userAgent, ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authorization URL",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"provider": "google",
	})
}

// GoogleCallback handles Google OAuth callback
// GET /api/auth/oauth/google/callback
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing code or state parameter",
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	token, user, isNewDevice, err := h.oauthService.HandleCallback(
		models.OAuthProviderGoogle,
		code,
		state,
		userAgent,
		ipAddress,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "OAuth authentication failed",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
			"balance":  user.Balance,
			"is_admin": user.IsAdmin,
		},
		"token":         token,
		"is_new_device": isNewDevice,
		"provider":      "google",
	})
}

// GitHubLogin initiates GitHub OAuth flow
// GET /api/auth/oauth/github
func (h *OAuthHandler) GitHubLogin(c *gin.Context) {
	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	authURL, err := h.oauthService.GenerateAuthURL(models.OAuthProviderGitHub, userAgent, ipAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate authorization URL",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"provider": "github",
	})
}

// GitHubCallback handles GitHub OAuth callback
// GET /api/auth/oauth/github/callback
func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing code or state parameter",
		})
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	token, user, isNewDevice, err := h.oauthService.HandleCallback(
		models.OAuthProviderGitHub,
		code,
		state,
		userAgent,
		ipAddress,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "OAuth authentication failed",
			"detail": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"username": user.Username,
			"balance":  user.Balance,
			"is_admin": user.IsAdmin,
		},
		"token":         token,
		"is_new_device": isNewDevice,
		"provider":      "github",
	})
}
