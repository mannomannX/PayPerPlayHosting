package service

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"gorm.io/gorm"
)

// AuthService handles authentication logic
type AuthService struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

// NewAuthService creates a new auth service
func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	IsAdmin bool  `json:"is_admin"`
	jwt.RegisteredClaims
}

// Register creates a new user account
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
		Email:    email,
		Username: username,
		Balance:  0.0,
		IsActive: true,
		IsAdmin:  false,
	}

	// Hash password
	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	// Save to database
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(email, password string) (string, *models.User, error) {
	// Find user by email
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, models.ErrInvalidCredentials
		}
		return "", nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return "", nil, errors.New("account is deactivated")
	}

	// Verify password
	if !user.CheckPassword(password) {
		return "", nil, models.ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
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
