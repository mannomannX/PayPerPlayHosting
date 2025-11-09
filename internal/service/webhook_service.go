package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// WebhookService handles Discord webhook operations
type WebhookService struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewWebhookService creates a new webhook service
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetWebhook returns a webhook configuration for a server
func (s *WebhookService) GetWebhook(serverID uint) (*models.ServerWebhook, error) {
	var webhook models.ServerWebhook
	if err := s.db.Where("server_id = ?", serverID).First(&webhook).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No webhook configured
		}
		return nil, err
	}
	return &webhook, nil
}

// CreateWebhook creates a new webhook configuration
func (s *WebhookService) CreateWebhook(serverID uint, webhookURL string) (*models.ServerWebhook, error) {
	// Check if webhook already exists
	existing, err := s.GetWebhook(serverID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("webhook already exists for this server")
	}

	webhook := &models.ServerWebhook{
		ServerID:        serverID,
		WebhookURL:      webhookURL,
		Enabled:         true,
		OnServerStart:   true,
		OnServerStop:    true,
		OnServerCrash:   true,
		OnPlayerJoin:    true,
		OnPlayerLeave:   true,
		OnBackupCreated: false,
	}

	if err := s.db.Create(webhook).Error; err != nil {
		return nil, err
	}

	return webhook, nil
}

// UpdateWebhook updates an existing webhook configuration
func (s *WebhookService) UpdateWebhook(serverID uint, updates map[string]interface{}) (*models.ServerWebhook, error) {
	webhook, err := s.GetWebhook(serverID)
	if err != nil {
		return nil, err
	}
	if webhook == nil {
		return nil, fmt.Errorf("webhook not found for server %d", serverID)
	}

	if err := s.db.Model(webhook).Updates(updates).Error; err != nil {
		return nil, err
	}

	return webhook, nil
}

// DeleteWebhook deletes a webhook configuration
func (s *WebhookService) DeleteWebhook(serverID uint) error {
	return s.db.Where("server_id = ?", serverID).Delete(&models.ServerWebhook{}).Error
}

// TestWebhook sends a test message to the webhook URL
func (s *WebhookService) TestWebhook(webhookURL string, serverName string) error {
	payload := models.DiscordWebhookPayload{
		Username: "PayPerPlay",
		Embeds: []models.DiscordEmbed{
			{
				Title:       "🔔 Test Webhook",
				Description: fmt.Sprintf("Webhook test for server **%s**", serverName),
				Color:       3447003, // Blue
				Footer: &models.DiscordEmbedFooter{
					Text: "PayPerPlay Hosting",
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	return s.sendWebhook(webhookURL, payload)
}

// SendEvent sends a server event to Discord webhook
func (s *WebhookService) SendEvent(data models.WebhookEventData) error {
	// Get webhook for this server
	webhook, err := s.GetWebhook(data.ServerID)
	if err != nil {
		logger.Error("Failed to get webhook", err, map[string]interface{}{
			"server_id": data.ServerID,
		})
		return err
	}

	if webhook == nil || !webhook.Enabled {
		return nil // No webhook or disabled
	}

	// Check if this event type is enabled
	if !s.isEventEnabled(webhook, data.EventType) {
		return nil // Event type disabled
	}

	// Build Discord embed
	embed := s.buildEmbed(data)
	payload := models.DiscordWebhookPayload{
		Username: "PayPerPlay",
		Embeds:   []models.DiscordEmbed{embed},
	}

	// Send webhook
	if err := s.sendWebhook(webhook.WebhookURL, payload); err != nil {
		logger.Error("Failed to send webhook", err, map[string]interface{}{
			"server_id":  data.ServerID,
			"event_type": data.EventType,
		})
		return err
	}

	logger.Info("Webhook sent", map[string]interface{}{
		"server_id":  data.ServerID,
		"event_type": data.EventType,
	})

	return nil
}

// isEventEnabled checks if an event type is enabled for this webhook
func (s *WebhookService) isEventEnabled(webhook *models.ServerWebhook, eventType models.WebhookEvent) bool {
	switch eventType {
	case models.WebhookEventServerStart:
		return webhook.OnServerStart
	case models.WebhookEventServerStop:
		return webhook.OnServerStop
	case models.WebhookEventServerCrash:
		return webhook.OnServerCrash
	case models.WebhookEventPlayerJoin:
		return webhook.OnPlayerJoin
	case models.WebhookEventPlayerLeave:
		return webhook.OnPlayerLeave
	case models.WebhookEventBackupCreated:
		return webhook.OnBackupCreated
	default:
		return false
	}
}

// buildEmbed creates a Discord embed for an event
func (s *WebhookService) buildEmbed(data models.WebhookEventData) models.DiscordEmbed {
	var title, description string
	var color int

	switch data.EventType {
	case models.WebhookEventServerStart:
		title = "🟢 Server Started"
		description = fmt.Sprintf("Server **%s** is now online!", data.ServerName)
		color = 3066993 // Green
	case models.WebhookEventServerStop:
		title = "🔴 Server Stopped"
		description = fmt.Sprintf("Server **%s** has been stopped.", data.ServerName)
		color = 15158332 // Red
	case models.WebhookEventServerCrash:
		title = "💥 Server Crashed"
		description = fmt.Sprintf("Server **%s** has crashed!", data.ServerName)
		if data.Message != "" {
			description += fmt.Sprintf("\n\n**Error:** %s", data.Message)
		}
		color = 15105570 // Dark Red
	case models.WebhookEventPlayerJoin:
		title = "👋 Player Joined"
		description = fmt.Sprintf("**%s** joined **%s**", data.PlayerName, data.ServerName)
		color = 3447003 // Blue
	case models.WebhookEventPlayerLeave:
		title = "👋 Player Left"
		description = fmt.Sprintf("**%s** left **%s**", data.PlayerName, data.ServerName)
		color = 10070709 // Grey
	case models.WebhookEventBackupCreated:
		title = "💾 Backup Created"
		description = fmt.Sprintf("Backup created for **%s**", data.ServerName)
		if data.Message != "" {
			description += fmt.Sprintf("\n\n**Size:** %s", data.Message)
		}
		color = 3447003 // Blue
	default:
		title = "📢 Server Event"
		description = fmt.Sprintf("Event on server **%s**", data.ServerName)
		color = 3447003 // Blue
	}

	return models.DiscordEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Footer: &models.DiscordEmbedFooter{
			Text: "PayPerPlay Hosting",
		},
		Timestamp: data.Timestamp.Format(time.RFC3339),
	}
}

// sendWebhook sends a webhook payload to Discord
func (s *WebhookService) sendWebhook(webhookURL string, payload models.DiscordWebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// NotifyServerStart sends a server start notification
func (s *WebhookService) NotifyServerStart(serverID uint, serverName string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventServerStart,
		Timestamp:  time.Now(),
	})
}

// NotifyServerStop sends a server stop notification
func (s *WebhookService) NotifyServerStop(serverID uint, serverName string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventServerStop,
		Timestamp:  time.Now(),
	})
}

// NotifyServerCrash sends a server crash notification
func (s *WebhookService) NotifyServerCrash(serverID uint, serverName string, errorMsg string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventServerCrash,
		Message:    errorMsg,
		Timestamp:  time.Now(),
	})
}

// NotifyPlayerJoin sends a player join notification
func (s *WebhookService) NotifyPlayerJoin(serverID uint, serverName string, playerName string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventPlayerJoin,
		PlayerName: playerName,
		Timestamp:  time.Now(),
	})
}

// NotifyPlayerLeave sends a player leave notification
func (s *WebhookService) NotifyPlayerLeave(serverID uint, serverName string, playerName string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventPlayerLeave,
		PlayerName: playerName,
		Timestamp:  time.Now(),
	})
}

// NotifyBackupCreated sends a backup created notification
func (s *WebhookService) NotifyBackupCreated(serverID uint, serverName string, backupSize string) {
	go s.SendEvent(models.WebhookEventData{
		ServerID:   serverID,
		ServerName: serverName,
		EventType:  models.WebhookEventBackupCreated,
		Message:    backupSize,
		Timestamp:  time.Now(),
	})
}

// GetWebhookRepository returns a webhook repository
func GetWebhookRepository() *WebhookRepository {
	return &WebhookRepository{db: repository.GetDB()}
}

// WebhookRepository handles database operations for webhooks
type WebhookRepository struct {
	db *gorm.DB
}

// FindByServerID finds a webhook by server ID
func (r *WebhookRepository) FindByServerID(serverID uint) (*models.ServerWebhook, error) {
	var webhook models.ServerWebhook
	if err := r.db.Where("server_id = ?", serverID).First(&webhook).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &webhook, nil
}

// Create creates a new webhook
func (r *WebhookRepository) Create(webhook *models.ServerWebhook) error {
	return r.db.Create(webhook).Error
}

// Update updates a webhook
func (r *WebhookRepository) Update(webhook *models.ServerWebhook) error {
	return r.db.Save(webhook).Error
}

// Delete deletes a webhook
func (r *WebhookRepository) Delete(serverID uint) error {
	return r.db.Where("server_id = ?", serverID).Delete(&models.ServerWebhook{}).Error
}
