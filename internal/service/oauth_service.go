package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// OAuthService handles OAuth authentication flows
type OAuthService struct {
	db              *gorm.DB
	userRepo        *repository.UserRepository
	cfg             *config.Config
	securityService *SecurityService
	emailService    *EmailService
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(db *gorm.DB, userRepo *repository.UserRepository, cfg *config.Config, securityService *SecurityService, emailService *EmailService) *OAuthService {
	return &OAuthService{
		db:              db,
		userRepo:        userRepo,
		cfg:             cfg,
		securityService: securityService,
		emailService:    emailService,
	}
}

// OAuthConfig holds provider-specific configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

// GetProviderConfig returns OAuth configuration for a provider
func (s *OAuthService) GetProviderConfig(provider models.OAuthProviderType) (*OAuthConfig, error) {
	switch provider {
	case models.OAuthProviderDiscord:
		return &OAuthConfig{
			ClientID:     s.cfg.DiscordClientID,
			ClientSecret: s.cfg.DiscordClientSecret,
			RedirectURL:  s.cfg.BaseURL + "/api/auth/oauth/discord/callback",
			AuthURL:      "https://discord.com/api/oauth2/authorize",
			TokenURL:     "https://discord.com/api/oauth2/token",
			UserInfoURL:  "https://discord.com/api/users/@me",
			Scopes:       []string{"identify", "email"},
		}, nil

	case models.OAuthProviderGoogle:
		return &OAuthConfig{
			ClientID:     s.cfg.GoogleClientID,
			ClientSecret: s.cfg.GoogleClientSecret,
			RedirectURL:  s.cfg.BaseURL + "/api/auth/oauth/google/callback",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		}, nil

	case models.OAuthProviderGitHub:
		return &OAuthConfig{
			ClientID:     s.cfg.GitHubClientID,
			ClientSecret: s.cfg.GitHubClientSecret,
			RedirectURL:  s.cfg.BaseURL + "/api/auth/oauth/github/callback",
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       []string{"user:email"},
		}, nil

	default:
		return nil, errors.New("unsupported OAuth provider")
	}
}

// GenerateAuthURL generates the OAuth authorization URL
func (s *OAuthService) GenerateAuthURL(provider models.OAuthProviderType, userAgent, ipAddress string) (string, error) {
	providerCfg, err := s.GetProviderConfig(provider)
	if err != nil {
		return "", err
	}

	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		return "", err
	}

	// Store state in database with expiration
	oauthState := &models.OAuthState{
		State:     state,
		Provider:  provider,
		ExpiresAt: time.Now().Add(10 * time.Minute),
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}

	if err := s.db.Create(oauthState).Error; err != nil {
		return "", err
	}

	// Build authorization URL
	authURL, err := url.Parse(providerCfg.AuthURL)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Add("client_id", providerCfg.ClientID)
	params.Add("redirect_uri", providerCfg.RedirectURL)
	params.Add("response_type", "code")
	params.Add("state", state)
	params.Add("scope", joinScopes(providerCfg.Scopes))

	authURL.RawQuery = params.Encode()

	logger.Info("OAuth authorization URL generated", map[string]interface{}{
		"provider": provider,
		"state":    state[:8] + "...",
	})

	return authURL.String(), nil
}

// HandleCallback handles the OAuth callback
func (s *OAuthService) HandleCallback(provider models.OAuthProviderType, code, state, userAgent, ipAddress string) (string, *models.User, bool, error) {
	// Verify state (CSRF protection)
	var oauthState models.OAuthState
	if err := s.db.Where("state = ? AND provider = ?", state, provider).First(&oauthState).Error; err != nil {
		logger.Warn("Invalid OAuth state", map[string]interface{}{
			"provider": provider,
			"error":    err.Error(),
		})
		return "", nil, false, errors.New("invalid or expired OAuth state")
	}

	// Delete used state
	s.db.Delete(&oauthState)

	// Check state expiration
	if oauthState.IsExpired() {
		return "", nil, false, errors.New("OAuth state expired")
	}

	// Exchange code for access token
	tokenResp, err := s.exchangeCodeForToken(provider, code)
	if err != nil {
		return "", nil, false, err
	}

	// Get user info from provider
	userInfo, err := s.getUserInfo(provider, tokenResp.AccessToken)
	if err != nil {
		return "", nil, false, err
	}

	// Find or create user
	user, isNewUser, isNewDevice, err := s.findOrCreateUser(provider, userInfo, tokenResp, userAgent, ipAddress)
	if err != nil {
		return "", nil, false, err
	}

	// Generate JWT token
	authService := &AuthService{
		userRepo:        s.userRepo,
		cfg:             s.cfg,
		emailService:    s.emailService,
		securityService: s.securityService,
	}
	token, err := authService.GenerateToken(user)
	if err != nil {
		return "", nil, false, err
	}

	// Log security event
	eventType := models.EventLoginSuccess
	if isNewUser {
		eventType = models.EventLoginNewDevice // New account = new device
	}
	_ = s.securityService.LogSecurityEvent(user.ID, eventType, ipAddress, userAgent, true, fmt.Sprintf("OAuth login via %s", provider))

	// Send security alert if new device (and not new user - they'll get welcome email)
	if isNewDevice && !isNewUser {
		deviceName := extractDeviceName(userAgent)
		_ = s.securityService.SendNewDeviceAlert(user, deviceName, ipAddress)
	}

	// Send welcome email for new users
	if isNewUser {
		_ = s.emailService.SendWelcomeEmail(user.Email, user.Username)
	}

	logger.Info("OAuth login successful", map[string]interface{}{
		"provider":     provider,
		"user_id":      user.ID,
		"is_new_user":  isNewUser,
		"is_new_device": isNewDevice,
	})

	return token, user, isNewDevice, nil
}

// TokenResponse represents OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// exchangeCodeForToken exchanges authorization code for access token
func (s *OAuthService) exchangeCodeForToken(provider models.OAuthProviderType, code string) (*TokenResponse, error) {
	providerCfg, err := s.GetProviderConfig(provider)
	if err != nil {
		return nil, err
	}

	// Prepare form data
	data := url.Values{}
	data.Set("client_id", providerCfg.ClientID)
	data.Set("client_secret", providerCfg.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", providerCfg.RedirectURL)

	// Make POST request
	req, err := http.NewRequestWithContext(context.Background(), "POST", providerCfg.TokenURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.URL.RawQuery = data.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("OAuth token exchange failed", errors.New("non-200 status"), map[string]interface{}{
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// OAuthUserInfo represents user information from OAuth provider
type OAuthUserInfo struct {
	ID        string
	Email     string
	Username  string
	AvatarURL string
	Verified  bool
}

// getUserInfo fetches user information from OAuth provider
func (s *OAuthService) getUserInfo(provider models.OAuthProviderType, accessToken string) (*OAuthUserInfo, error) {
	providerCfg, err := s.GetProviderConfig(provider)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", providerCfg.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed with status %d", resp.StatusCode)
	}

	var rawData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return nil, err
	}

	// Parse provider-specific response
	return s.parseUserInfo(provider, rawData)
}

// parseUserInfo parses provider-specific user info response
func (s *OAuthService) parseUserInfo(provider models.OAuthProviderType, data map[string]interface{}) (*OAuthUserInfo, error) {
	userInfo := &OAuthUserInfo{}

	switch provider {
	case models.OAuthProviderDiscord:
		userInfo.ID = getStringField(data, "id")
		userInfo.Email = getStringField(data, "email")
		userInfo.Username = getStringField(data, "username")
		userInfo.Verified = getBoolField(data, "verified")
		if avatar := getStringField(data, "avatar"); avatar != "" {
			userInfo.AvatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", userInfo.ID, avatar)
		}

	case models.OAuthProviderGoogle:
		userInfo.ID = getStringField(data, "id")
		userInfo.Email = getStringField(data, "email")
		userInfo.Username = getStringField(data, "name")
		userInfo.AvatarURL = getStringField(data, "picture")
		userInfo.Verified = getBoolField(data, "verified_email")

	case models.OAuthProviderGitHub:
		userInfo.ID = fmt.Sprintf("%v", data["id"]) // GitHub uses numeric ID
		userInfo.Email = getStringField(data, "email")
		userInfo.Username = getStringField(data, "login")
		userInfo.AvatarURL = getStringField(data, "avatar_url")
		userInfo.Verified = true // GitHub doesn't provide verified field

	default:
		return nil, errors.New("unsupported provider")
	}

	return userInfo, nil
}

// findOrCreateUser finds existing user or creates new one from OAuth
func (s *OAuthService) findOrCreateUser(provider models.OAuthProviderType, userInfo *OAuthUserInfo, tokenResp *TokenResponse, userAgent, ipAddress string) (*models.User, bool, bool, error) {
	// Try to find existing OAuth account
	var oauthAccount models.OAuthAccount
	err := s.db.Where("provider = ? AND provider_id = ?", provider, userInfo.ID).
		Preload("User").
		First(&oauthAccount).Error

	if err == nil {
		// OAuth account exists - update and return user
		oauthAccount.AccessToken = tokenResp.AccessToken
		oauthAccount.RefreshToken = tokenResp.RefreshToken
		if tokenResp.ExpiresIn > 0 {
			expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
			oauthAccount.ExpiresAt = &expiresAt
		}
		oauthAccount.LastUsedAt = time.Now()
		s.db.Save(&oauthAccount)

		// Check for new device
		_, isTrusted := s.securityService.CheckTrustedDevice(oauthAccount.User.ID, userAgent, ipAddress)

		return &oauthAccount.User, false, !isTrusted, nil
	}

	// OAuth account doesn't exist - try to find user by email
	var user *models.User
	isNewUser := false

	if userInfo.Email != "" {
		user, err = s.userRepo.FindByEmail(userInfo.Email)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, false, err
		}
	}

	// Create new user if doesn't exist
	if user == nil {
		user = &models.User{
			Email:         userInfo.Email,
			Username:      userInfo.Username,
			Password:      generateRandomPassword(), // Random password for OAuth-only users
			EmailVerified: userInfo.Verified,        // OAuth providers verify emails
			IsActive:      true,
			IsAdmin:       false,
			Balance:       0.0,
		}

		if err := s.userRepo.Create(user); err != nil {
			return nil, false, false, err
		}

		isNewUser = true
	}

	// Create OAuth account link
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	newOAuthAccount := &models.OAuthAccount{
		UserID:       user.ID,
		Provider:     provider,
		ProviderID:   userInfo.ID,
		Email:        userInfo.Email,
		Username:     userInfo.Username,
		AvatarURL:    userInfo.AvatarURL,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    &expiresAt,
		LastUsedAt:   time.Now(),
	}

	if err := s.db.Create(newOAuthAccount).Error; err != nil {
		return nil, false, false, err
	}

	// Trust this device for new users (first login)
	if isNewUser {
		deviceName := fmt.Sprintf("%s Account", provider)
		_, _ = s.securityService.TrustNewDevice(user.ID, userAgent, ipAddress, deviceName)
	}

	return user, isNewUser, !isNewUser, nil
}

// Helper functions

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func generateRandomPassword() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func joinScopes(scopes []string) string {
	result := ""
	for i, scope := range scopes {
		if i > 0 {
			result += " "
		}
		result += scope
	}
	return result
}

func getStringField(data map[string]interface{}, field string) string {
	if val, ok := data[field]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBoolField(data map[string]interface{}, field string) bool {
	if val, ok := data[field]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}
