package service

import (
	"fmt"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
	// Uncomment when ready to use Resend:
	// "github.com/resendlabs/resend-go"
)

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendVerificationEmail(email, username, token string) error
	SendPasswordResetEmail(email, username, token string) error
	SendWelcomeEmail(email, username string) error
	SendAccountDeletedEmail(email, username string) error
	SendNewDeviceAlert(email, username, deviceName, ipAddress string, loginTime time.Time) error
	SendAccountLockedAlert(email, username string, lockDuration time.Duration) error
	SendPasswordChangedAlert(email, username string) error
}

// EmailService manages email sending
type EmailService struct {
	sender EmailSender
	db     *gorm.DB
}

// NewEmailService creates a new email service
func NewEmailService(sender EmailSender, db *gorm.DB) *EmailService {
	return &EmailService{
		sender: sender,
		db:     db,
	}
}

// SendVerificationEmail sends an email verification link
func (s *EmailService) SendVerificationEmail(email, username, token string) error {
	return s.sender.SendVerificationEmail(email, username, token)
}

// SendPasswordResetEmail sends a password reset link
func (s *EmailService) SendPasswordResetEmail(email, username, token string) error {
	return s.sender.SendPasswordResetEmail(email, username, token)
}

// SendWelcomeEmail sends a welcome email after registration
func (s *EmailService) SendWelcomeEmail(email, username string) error {
	return s.sender.SendWelcomeEmail(email, username)
}

// SendAccountDeletedEmail sends a confirmation email after account deletion
func (s *EmailService) SendAccountDeletedEmail(email, username string) error {
	return s.sender.SendAccountDeletedEmail(email, username)
}

// SendNewDeviceAlert sends an alert for a new device login
func (s *EmailService) SendNewDeviceAlert(email, username, deviceName, ipAddress string, loginTime time.Time) error {
	return s.sender.SendNewDeviceAlert(email, username, deviceName, ipAddress, loginTime)
}

// SendAccountLockedAlert sends an alert when account is locked
func (s *EmailService) SendAccountLockedAlert(email, username string, lockDuration time.Duration) error {
	return s.sender.SendAccountLockedAlert(email, username, lockDuration)
}

// SendPasswordChangedAlert sends an alert when password is changed
func (s *EmailService) SendPasswordChangedAlert(email, username string) error {
	return s.sender.SendPasswordChangedAlert(email, username)
}

// ========================================
// 🚧 MOCK EMAIL SENDER - REPLACE WITH REAL SMTP LATER
// ========================================

// MockEmailSender simulates email sending by logging to console and database
type MockEmailSender struct {
	db *gorm.DB
}

// MockEmail stores simulated emails in database for testing
type MockEmail struct {
	ID        uint      `gorm:"primaryKey"`
	To        string    `gorm:"size:255"`
	Subject   string    `gorm:"size:500"`
	Body      string    `gorm:"type:text"`
	Type      string    `gorm:"size:50"` // verification, password_reset, welcome, etc.
	CreatedAt time.Time
}

// NewMockEmailSender creates a mock email sender
func NewMockEmailSender(db *gorm.DB) *MockEmailSender {
	// Auto-migrate mock emails table
	db.AutoMigrate(&MockEmail{})
	return &MockEmailSender{db: db}
}

// SendVerificationEmail simulates sending verification email
func (m *MockEmailSender) SendVerificationEmail(email, username, token string) error {
	verificationLink := fmt.Sprintf("http://localhost:3000/verify-email?token=%s", token)

	body := fmt.Sprintf(`
Hi %s,

Welcome to PayPerPlay! Please verify your email address by clicking the link below:

%s

This link will expire in 24 hours.

If you didn't create an account, please ignore this email.

Best regards,
PayPerPlay Team
	`, username, verificationLink)

	mockEmail := &MockEmail{
		To:      email,
		Subject: "Verify your PayPerPlay account",
		Body:    body,
		Type:    "verification",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("📧 MOCK EMAIL SENT (Verification)", map[string]interface{}{
		"to":      email,
		"subject": mockEmail.Subject,
		"link":    verificationLink,
		"note":    "🚧 This is a simulated email. Replace MockEmailSender with SMTP implementation.",
	})

	return nil
}

// SendPasswordResetEmail simulates sending password reset email
func (m *MockEmailSender) SendPasswordResetEmail(email, username, token string) error {
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token)

	body := fmt.Sprintf(`
Hi %s,

We received a request to reset your password. Click the link below to set a new password:

%s

This link will expire in 1 hour.

If you didn't request a password reset, please ignore this email and your password will remain unchanged.

Best regards,
PayPerPlay Team
	`, username, resetLink)

	mockEmail := &MockEmail{
		To:      email,
		Subject: "Reset your PayPerPlay password",
		Body:    body,
		Type:    "password_reset",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("📧 MOCK EMAIL SENT (Password Reset)", map[string]interface{}{
		"to":      email,
		"subject": mockEmail.Subject,
		"link":    resetLink,
		"note":    "🚧 This is a simulated email. Replace MockEmailSender with SMTP implementation.",
	})

	return nil
}

// SendWelcomeEmail simulates sending welcome email
func (m *MockEmailSender) SendWelcomeEmail(email, username string) error {
	body := fmt.Sprintf(`
Hi %s,

Welcome to PayPerPlay! 🎉

Your account has been successfully verified. You can now create your first Minecraft server.

Getting Started:
1. Create a new server from templates
2. Configure your server settings
3. Start playing!

Need help? Check out our documentation or join our Discord community.

Best regards,
PayPerPlay Team
	`, username)

	mockEmail := &MockEmail{
		To:      email,
		Subject: "Welcome to PayPerPlay!",
		Body:    body,
		Type:    "welcome",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("📧 MOCK EMAIL SENT (Welcome)", map[string]interface{}{
		"to":      email,
		"subject": mockEmail.Subject,
		"note":    "🚧 This is a simulated email. Replace MockEmailSender with SMTP implementation.",
	})

	return nil
}

// SendAccountDeletedEmail simulates sending account deletion confirmation
func (m *MockEmailSender) SendAccountDeletedEmail(email, username string) error {
	body := fmt.Sprintf(`
Hi %s,

Your PayPerPlay account has been successfully deleted.

All your servers and data have been permanently removed as per your request.

If you didn't request this deletion or believe this was a mistake, please contact our support team immediately.

We're sorry to see you go. If you change your mind, you're always welcome to create a new account.

Best regards,
PayPerPlay Team
	`, username)

	mockEmail := &MockEmail{
		To:      email,
		Subject: "Your PayPerPlay account has been deleted",
		Body:    body,
		Type:    "account_deleted",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("📧 MOCK EMAIL SENT (Account Deleted)", map[string]interface{}{
		"to":      email,
		"subject": mockEmail.Subject,
		"note":    "🚧 This is a simulated email. Replace MockEmailSender with SMTP implementation.",
	})

	return nil
}

// SendNewDeviceAlert simulates sending new device login alert
func (m *MockEmailSender) SendNewDeviceAlert(email, username, deviceName, ipAddress string, loginTime time.Time) error {
	body := fmt.Sprintf(`
🔒 SECURITY ALERT: New Device Login

Hi %s,

We detected a login to your PayPerPlay account from a new device:

Device: %s
IP Address: %s
Time: %s

If this was you, you can safely ignore this email. The device will be trusted for 30 days.

If you don't recognize this activity, please:
1. Change your password immediately
2. Review your account security settings
3. Contact support if you need assistance

Best regards,
PayPerPlay Security Team
	`, username, deviceName, ipAddress, loginTime.Format("2006-01-02 15:04:05 MST"))

	mockEmail := &MockEmail{
		To:      email,
		Subject: "🔒 New device login detected",
		Body:    body,
		Type:    "security_alert_new_device",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("🔒 MOCK SECURITY ALERT (New Device)", map[string]interface{}{
		"to":      email,
		"device":  deviceName,
		"ip":      ipAddress,
		"note":    "🚧 This is a simulated security alert.",
	})

	return nil
}

// SendAccountLockedAlert simulates sending account locked alert
func (m *MockEmailSender) SendAccountLockedAlert(email, username string, lockDuration time.Duration) error {
	body := fmt.Sprintf(`
🔒 SECURITY ALERT: Account Temporarily Locked

Hi %s,

Your PayPerPlay account has been temporarily locked due to multiple failed login attempts.

Lock Duration: %s

This is a security measure to protect your account. You can try logging in again after the lock expires.

If you didn't attempt to log in, your account may be under attack. Please:
1. Change your password immediately after the lock expires
2. Enable two-factor authentication (when available)
3. Contact support if you need immediate assistance

Best regards,
PayPerPlay Security Team
	`, username, lockDuration.String())

	mockEmail := &MockEmail{
		To:      email,
		Subject: "🔒 Your account has been temporarily locked",
		Body:    body,
		Type:    "security_alert_account_locked",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("🔒 MOCK SECURITY ALERT (Account Locked)", map[string]interface{}{
		"to":       email,
		"duration": lockDuration.String(),
		"note":     "🚧 This is a simulated security alert.",
	})

	return nil
}

// SendPasswordChangedAlert simulates sending password changed alert
func (m *MockEmailSender) SendPasswordChangedAlert(email, username string) error {
	body := fmt.Sprintf(`
🔒 SECURITY ALERT: Password Changed

Hi %s,

Your PayPerPlay account password was successfully changed just now.

If you made this change, you can safely ignore this email.

If you DID NOT change your password, your account may have been compromised. Please:
1. Try to log in and change your password immediately
2. Contact support immediately for assistance
3. Review your recent account activity

Best regards,
PayPerPlay Security Team
	`, username)

	mockEmail := &MockEmail{
		To:      email,
		Subject: "🔒 Your password was changed",
		Body:    body,
		Type:    "security_alert_password_changed",
	}

	if err := m.db.Create(mockEmail).Error; err != nil {
		return err
	}

	// 🚧 TODO: Replace with real email service
	logger.Info("🔒 MOCK SECURITY ALERT (Password Changed)", map[string]interface{}{
		"to":   email,
		"note": "🚧 This is a simulated security alert.",
	})

	return nil
}

// ========================================
// 🚀 RESEND EMAIL SENDER - PRODUCTION READY
// ========================================
// Uncomment this when you're ready to use Resend API
// Steps:
// 1. Sign up at https://resend.com (free tier: 3,000 emails/month)
// 2. Get your API key from dashboard
// 3. Add to .env: RESEND_API_KEY=re_xxxxx
// 4. Run: go get github.com/resendlabs/resend-go
// 5. Uncomment the code below
// 6. In main.go, replace NewMockEmailSender with NewResendEmailSender

/*
type ResendEmailSender struct {
	client *resend.Client
	fromEmail string
	frontendURL string
}

// NewResendEmailSender creates a production email sender using Resend API
func NewResendEmailSender(apiKey, fromEmail, frontendURL string) *ResendEmailSender {
	client := resend.NewClient(apiKey)
	return &ResendEmailSender{
		client: client,
		fromEmail: fromEmail, // e.g., "PayPerPlay <noreply@payperplay.host>"
		frontendURL: frontendURL, // e.g., "https://payperplay.host"
	}
}

// SendVerificationEmail sends verification email via Resend
func (r *ResendEmailSender) SendVerificationEmail(email, username, token string) error {
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", r.frontendURL, token)

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #4CAF50;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin: 20px 0;
        }
        .footer { margin-top: 30px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Welcome to PayPerPlay! 🎮</h2>
        <p>Hi %s,</p>
        <p>Thanks for signing up! Please verify your email address to get started:</p>
        <a href="%s" class="button">Verify Email Address</a>
        <p>Or copy and paste this link into your browser:<br>
        <code>%s</code></p>
        <p>This link will expire in 24 hours.</p>
        <div class="footer">
            <p>If you didn't create an account, you can safely ignore this email.</p>
            <p>© 2025 PayPerPlay - Pay only when you play</p>
        </div>
    </div>
</body>
</html>
	`, username, verificationLink, verificationLink)

	params := &resend.SendEmailRequest{
		From:    r.fromEmail,
		To:      []string{email},
		Subject: "Verify your PayPerPlay account",
		Html:    htmlBody,
	}

	_, err := r.client.Emails.Send(params)
	if err != nil {
		logger.Error("Failed to send verification email via Resend", err, map[string]interface{}{
			"to": email,
		})
		return err
	}

	logger.Info("✅ Verification email sent via Resend", map[string]interface{}{
		"to": email,
	})
	return nil
}

// SendPasswordResetEmail sends password reset email via Resend
func (r *ResendEmailSender) SendPasswordResetEmail(email, username, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", r.frontendURL, token)

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #FF9800;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            margin: 20px 0;
        }
        .warning { background-color: #fff3cd; padding: 12px; border-left: 4px solid #ff9800; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Reset Your Password 🔑</h2>
        <p>Hi %s,</p>
        <p>We received a request to reset your password. Click the button below to set a new password:</p>
        <a href="%s" class="button">Reset Password</a>
        <p>Or copy and paste this link into your browser:<br>
        <code>%s</code></p>
        <div class="warning">
            ⚠️ This link will expire in 1 hour.
        </div>
        <p>If you didn't request a password reset, please ignore this email. Your password will remain unchanged.</p>
        <p>© 2025 PayPerPlay</p>
    </div>
</body>
</html>
	`, username, resetLink, resetLink)

	params := &resend.SendEmailRequest{
		From:    r.fromEmail,
		To:      []string{email},
		Subject: "Reset your PayPerPlay password",
		Html:    htmlBody,
	}

	_, err := r.client.Emails.Send(params)
	return err
}

// SendWelcomeEmail sends welcome email via Resend
func (r *ResendEmailSender) SendWelcomeEmail(email, username string) error {
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2>Welcome to PayPerPlay! 🎉</h2>
        <p>Hi %s,</p>
        <p>Your account has been successfully verified! You're now ready to create your first Minecraft server.</p>
        <h3>Getting Started:</h3>
        <ol>
            <li>Create a new server from our templates</li>
            <li>Configure your server settings</li>
            <li>Invite your friends and start playing!</li>
        </ol>
        <p>Remember: You only pay when your server is running. Stopped servers cost nothing!</p>
        <a href="%s/dashboard" style="display: inline-block; padding: 12px 24px; background-color: #4CAF50; color: white; text-decoration: none; border-radius: 4px; margin: 20px 0;">
            Go to Dashboard
        </a>
        <p>Need help? Join our <a href="https://discord.gg/payperplay">Discord community</a> or check the <a href="%s/docs">documentation</a>.</p>
        <p>Happy gaming! 🎮<br>The PayPerPlay Team</p>
    </div>
</body>
</html>
	`, username, r.frontendURL, r.frontendURL)

	params := &resend.SendEmailRequest{
		From:    r.fromEmail,
		To:      []string{email},
		Subject: "Welcome to PayPerPlay! 🎉",
		Html:    htmlBody,
	}

	_, err := r.client.Emails.Send(params)
	return err
}

// SendAccountDeletedEmail sends account deletion confirmation via Resend
func (r *ResendEmailSender) SendAccountDeletedEmail(email, username string) error {
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px;">
        <h2>Account Deleted</h2>
        <p>Hi %s,</p>
        <p>Your PayPerPlay account has been successfully deleted.</p>
        <p>All your servers and data have been permanently removed as per your request.</p>
        <div style="background-color: #f8d7da; padding: 12px; border-left: 4px solid #dc3545; margin: 20px 0;">
            ⚠️ If you didn't request this deletion, please contact our support team immediately at support@payperplay.host
        </div>
        <p>We're sorry to see you go. If you change your mind, you're always welcome to create a new account.</p>
        <p>Best regards,<br>The PayPerPlay Team</p>
    </div>
</body>
</html>
	`, username)

	params := &resend.SendEmailRequest{
		From:    r.fromEmail,
		To:      []string{email},
		Subject: "Your PayPerPlay account has been deleted",
		Html:    htmlBody,
	}

	_, err := r.client.Emails.Send(params)
	return err
}
*/
