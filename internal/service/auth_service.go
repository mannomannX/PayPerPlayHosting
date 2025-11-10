package service

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"gorm.io/gorm"
)

// AuthService handles authentication logic
type AuthService struct {
	userRepo        *repository.UserRepository
	cfg             *config.Config
	emailService    *EmailService
	securityService *SecurityService
}

// NewAuthService creates a new auth service
func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config, emailService *EmailService, securityService *SecurityService) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		cfg:             cfg,
		emailService:    emailService,
		securityService: securityService,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	IsAdmin bool  `json:"is_admin"`
	jwt.RegisteredClaims
}

// Register creates a new user account and sends verification email
func (s *AuthService) Register(email, password, username string) (*models.User, error) {
	// Check if email already exists
	_, err := s.userRepo.FindByEmail(email)
	if err == nil {
		return nil, models.ErrEmailAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create user
	user := &models.User{
		Email:         email,
		Username:      username,
		Balance:       0.0,
		IsActive:      true,
		IsAdmin:       false,
		EmailVerified: false, // Not verified yet
	}

	// Hash password
	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	// Generate email verification token
	verificationToken := user.GenerateVerificationToken()

	// Save to database
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// Send verification email
	if err := s.emailService.SendVerificationEmail(user.Email, user.Username, verificationToken); err != nil {
		// Log error but don't fail registration
		// User can request a new verification email later
		return user, nil
	}

	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(email, password, userAgent, ipAddress string) (string, *models.User, bool, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, false, models.ErrInvalidCredentials
		}
		return "", nil, false, err
	}

	// Check if user is active
	if !user.IsActive {
		return "", nil, false, errors.New("account is deactivated")
	}

	// Check if email is verified
	if !user.EmailVerified {
		return "", nil, false, models.ErrEmailNotVerified
	}

	// Check if account is locked
	if user.IsLocked() {
		// Log failed login attempt (account locked)
		_ = s.securityService.LogSecurityEvent(user.ID, models.EventLoginFailure, ipAddress, userAgent, false, "Account is locked")
		return "", nil, false, models.ErrAccountLocked
	}

	// Verify password
	if !user.CheckPassword(password) {
		// Increment failed login attempts
		lockDuration := user.IncrementFailedLogins()
		if err := s.userRepo.Update(user); err != nil {
			return "", nil, false, err
		}

		// Log failed login attempt
		_ = s.securityService.LogSecurityEvent(user.ID, models.EventLoginFailure, ipAddress, userAgent, false, "Invalid password")

		// If account just got locked, send alert
		if lockDuration > 0 {
			_ = s.securityService.LogSecurityEvent(user.ID, models.EventAccountLocked, ipAddress, userAgent, true, "")
			_ = s.securityService.SendAccountLockedAlert(user, lockDuration)
		}

		return "", nil, false, models.ErrInvalidCredentials
	}

	// Check if this is a trusted device
	_, isTrusted := s.securityService.CheckTrustedDevice(user.ID, userAgent, ipAddress)

	// Successful login - reset failed login attempts
	user.ResetFailedLogins()
	if err := s.userRepo.Update(user); err != nil {
		return "", nil, false, err
	}

	// Log successful login
	if isTrusted {
		_ = s.securityService.LogSecurityEvent(user.ID, models.EventLoginSuccess, ipAddress, userAgent, true, "Trusted device")
	} else {
		// New device - log and send alert
		_ = s.securityService.LogSecurityEvent(user.ID, models.EventLoginNewDevice, ipAddress, userAgent, true, "")
		deviceName := extractDeviceName(userAgent)
		_ = s.securityService.SendNewDeviceAlert(user, deviceName, ipAddress)
	}

	// Generate JWT token
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", nil, false, err
	}

	return token, user, !isTrusted, nil
}

// GenerateToken generates a JWT token for a user
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Token expires in 24 hours

	claims := &Claims{
		UserID:  user.ID,
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "payperplay",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(userID string) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}

// RefreshToken generates a new token for an existing valid token
func (s *AuthService) RefreshToken(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Get fresh user data
	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil {
		return "", err
	}

	// Generate new token
	return s.GenerateToken(user)
}

// ========================================
// Email Verification Methods
// ========================================

// VerifyEmail verifies a user's email with the provided token
func (s *AuthService) VerifyEmail(token string) (*models.User, error) {
	// Find user by verification token
	user, err := s.userRepo.FindByVerificationToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.ErrInvalidVerificationToken
		}
		return nil, err
	}

	// Check if already verified
	if user.EmailVerified {
		return nil, models.ErrEmailAlreadyVerified
	}

	// Validate token and expiry
	if !user.IsVerificationTokenValid(token) {
		return nil, models.ErrInvalidVerificationToken
	}

	// Mark email as verified
	user.MarkEmailVerified()

	// Save to database
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	// Send welcome email
	_ = s.emailService.SendWelcomeEmail(user.Email, user.Username)

	return user, nil
}

// ResendVerificationEmail sends a new verification email to the user
func (s *AuthService) ResendVerificationEmail(email string) error {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ErrInvalidCredentials
		}
		return err
	}

	// Check if already verified
	if user.EmailVerified {
		return models.ErrEmailAlreadyVerified
	}

	// Generate new verification token
	verificationToken := user.GenerateVerificationToken()

	// Save to database
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Send verification email
	return s.emailService.SendVerificationEmail(user.Email, user.Username, verificationToken)
}

// ========================================
// Password Reset Methods
// ========================================

// RequestPasswordReset initiates a password reset flow
func (s *AuthService) RequestPasswordReset(email string) error {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		// Don't reveal if email exists or not (security best practice)
		// Always return success to prevent email enumeration
		return nil
	}

	// Generate password reset token
	resetToken := user.GeneratePasswordResetToken()

	// Save to database
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Send password reset email
	return s.emailService.SendPasswordResetEmail(user.Email, user.Username, resetToken)
}

// ResetPassword resets a user's password using a valid reset token
func (s *AuthService) ResetPassword(token, newPassword string) error {
	// Find user by reset token
	user, err := s.userRepo.FindByResetToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ErrInvalidResetToken
		}
		return err
	}

	// Validate token and expiry
	if !user.IsResetTokenValid(token) {
		return models.ErrInvalidResetToken
	}

	// Set new password
	if err := user.SetPassword(newPassword); err != nil {
		return err
	}

	// Update password change timestamp
	user.UpdatePasswordChanged()

	// Clear reset token
	user.ClearPasswordResetToken()

	// Reset failed login attempts (password was reset successfully)
	user.ResetFailedLogins()

	// Save to database
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Log security event
	_ = s.securityService.LogSecurityEvent(user.ID, models.EventPasswordResetSuccess, "", "", true, "Password reset via email token")

	// Send security alert
	_ = s.securityService.SendPasswordChangedAlert(user)

	return nil
}

// ========================================
// Account Management Methods
// ========================================

// ChangePassword changes a user's password (when logged in)
func (s *AuthService) ChangePassword(userID, currentPassword, newPassword string) error {
	// Get user
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// Verify current password
	if !user.CheckPassword(currentPassword) {
		return models.ErrInvalidCredentials
	}

	// Set new password
	if err := user.SetPassword(newPassword); err != nil {
		return err
	}

	// Update password change timestamp
	user.UpdatePasswordChanged()

	// Save to database
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Log security event
	_ = s.securityService.LogSecurityEvent(userID, models.EventPasswordChanged, "", "", true, "User-initiated password change")

	// Send security alert
	_ = s.securityService.SendPasswordChangedAlert(user)

	return nil
}

// UpdateProfile updates user profile information
func (s *AuthService) UpdateProfile(userID, username string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	user.Username = username
	return s.userRepo.Update(user)
}

// DeleteAccount permanently deletes a user account and all associated data
func (s *AuthService) DeleteAccount(userID, password string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	// Require password re-authentication for account deletion
	if !user.CheckPassword(password) {
		return models.ErrInvalidCredentials
	}

	// Log security event
	_ = s.securityService.LogSecurityEvent(userID, models.EventAccountDeleted, "", "", true, "User-initiated account deletion")

	// Send account deleted confirmation email
	_ = s.emailService.SendAccountDeletedEmail(user.Email, user.Username)

	// Delete user (cascade will delete servers, sessions, etc.)
	return s.userRepo.Delete(userID)
}

// extractDeviceName extracts a human-readable device name from user agent
func extractDeviceName(userAgent string) string {
	ua := strings.ToLower(userAgent)

	// Detect browser
	browser := "Unknown Browser"
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browser = "Safari"
	} else if strings.Contains(ua, "edg") {
		browser = "Edge"
	} else if strings.Contains(ua, "opera") || strings.Contains(ua, "opr") {
		browser = "Opera"
	}

	// Detect OS
	os := "Unknown OS"
	if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		os = "macOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	} else if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		os = "iOS"
	}

	return browser + " on " + os
}
