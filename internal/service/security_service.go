package service

import (
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// SecurityService manages device trust and security events
type SecurityService struct {
	db           *gorm.DB
	emailService *EmailService
}

// NewSecurityService creates a new security service
func NewSecurityService(db *gorm.DB, emailService *EmailService) *SecurityService {
	return &SecurityService{
		db:           db,
		emailService: emailService,
	}
}

// CheckTrustedDevice checks if a device is trusted for this user
func (s *SecurityService) CheckTrustedDevice(userID, userAgent, ipAddress string) (*models.TrustedDevice, bool) {
	deviceID := models.GenerateDeviceID(userAgent, ipAddress)

	var device models.TrustedDevice
	err := s.db.Where("user_id = ? AND device_id = ? AND is_active = ? AND expires_at > ?",
		userID, deviceID, true, time.Now()).First(&device).Error

	if err != nil {
		return nil, false
	}

	// Renew trust for another 30 days
	device.Renew()
	s.db.Save(&device)

	return &device, true
}

// TrustNewDevice creates a new trusted device entry
func (s *SecurityService) TrustNewDevice(userID, userAgent, ipAddress, name string) (*models.TrustedDevice, error) {
	deviceID := models.GenerateDeviceID(userAgent, ipAddress)

	device := &models.TrustedDevice{
		UserID:    userID,
		DeviceID:  deviceID,
		Name:      name,
		UserAgent: userAgent,
		IPAddress: ipAddress,
		LastUsed:  time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		IsActive:  true,
	}

	if err := s.db.Create(device).Error; err != nil {
		return nil, err
	}

	logger.Info("New trusted device added", map[string]interface{}{
		"user_id":   userID,
		"device_id": deviceID,
		"name":      name,
	})

	return device, nil
}

// RemoveTrustedDevice removes a device from the trusted list
func (s *SecurityService) RemoveTrustedDevice(userID, deviceID string) error {
	return s.db.Where("user_id = ? AND device_id = ?", userID, deviceID).
		Update("is_active", false).Error
}

// GetUserDevices returns all active trusted devices for a user
func (s *SecurityService) GetUserDevices(userID string) ([]models.TrustedDevice, error) {
	var devices []models.TrustedDevice
	err := s.db.Where("user_id = ? AND is_active = ? AND expires_at > ?",
		userID, true, time.Now()).
		Order("last_used DESC").
		Find(&devices).Error

	return devices, err
}

// LogSecurityEvent logs a security event
func (s *SecurityService) LogSecurityEvent(userID string, eventType models.SecurityEventType, ipAddress, userAgent string, success bool, reason string) error {
	deviceID := models.GenerateDeviceID(userAgent, ipAddress)

	event := &models.SecurityEvent{
		UserID:    userID,
		EventType: eventType,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		DeviceID:  deviceID,
		Success:   success,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	if err := s.db.Create(event).Error; err != nil {
		logger.Error("Failed to log security event", err, map[string]interface{}{
			"user_id":    userID,
			"event_type": eventType,
		})
		return err
	}

	logger.Debug("Security event logged", map[string]interface{}{
		"user_id":    userID,
		"event_type": eventType,
		"success":    success,
	})

	return nil
}

// GetRecentSecurityEvents returns recent security events for a user
func (s *SecurityService) GetRecentSecurityEvents(userID string, limit int) ([]models.SecurityEvent, error) {
	var events []models.SecurityEvent
	err := s.db.Where("user_id = ?", userID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&events).Error

	return events, err
}

// SendNewDeviceAlert sends an email alert for a new device login
func (s *SecurityService) SendNewDeviceAlert(user *models.User, deviceName, ipAddress string) error {
	return s.emailService.SendNewDeviceAlert(user.Email, user.Username, deviceName, ipAddress, time.Now())
}

// SendAccountLockedAlert sends an email alert when account is locked
func (s *SecurityService) SendAccountLockedAlert(user *models.User, lockDuration time.Duration) error {
	return s.emailService.SendAccountLockedAlert(user.Email, user.Username, lockDuration)
}

// SendPasswordChangedAlert sends an email alert when password is changed
func (s *SecurityService) SendPasswordChangedAlert(user *models.User) error {
	return s.emailService.SendPasswordChangedAlert(user.Email, user.Username)
}

// CleanupExpiredDevices removes expired trusted devices (runs periodically)
func (s *SecurityService) CleanupExpiredDevices() error {
	result := s.db.Where("expires_at < ? OR is_active = ?", time.Now(), false).
		Delete(&models.TrustedDevice{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		logger.Info("Cleaned up expired devices", map[string]interface{}{
			"count": result.RowsAffected,
		})
	}

	return nil
}
