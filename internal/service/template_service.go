package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/pkg/logger"
)

// TemplateService handles server template operations
type TemplateService struct {
	templatesPath string
	templates     []models.ServerTemplate
	categories    []models.TemplateCategory
}

// TemplateData holds the entire templates JSON structure
type TemplateData struct {
	Templates  []models.ServerTemplate     `json:"templates"`
	Categories []models.TemplateCategory   `json:"categories"`
}

// NewTemplateService creates a new template service
func NewTemplateService(templatesPath string) (*TemplateService, error) {
	service := &TemplateService{
		templatesPath: templatesPath,
	}

	// Load templates on initialization
	if err := service.LoadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return service, nil
}

// LoadTemplates loads templates from JSON file
func (s *TemplateService) LoadTemplates() error {
	data, err := os.ReadFile(s.templatesPath)
	if err != nil {
		return fmt.Errorf("failed to read templates file: %w", err)
	}

	var templateData TemplateData
	if err := json.Unmarshal(data, &templateData); err != nil {
		return fmt.Errorf("failed to parse templates JSON: %w", err)
	}

	// Add timestamps if not present
	now := time.Now()
	for i := range templateData.Templates {
		if templateData.Templates[i].CreatedAt.IsZero() {
			templateData.Templates[i].CreatedAt = now
		}
		if templateData.Templates[i].UpdatedAt.IsZero() {
			templateData.Templates[i].UpdatedAt = now
		}
	}

	s.templates = templateData.Templates
	s.categories = templateData.Categories

	logger.Info("Templates loaded", map[string]interface{}{
		"count":      len(s.templates),
		"categories": len(s.categories),
	})

	return nil
}

// GetAllTemplates returns all available templates
func (s *TemplateService) GetAllTemplates() []models.ServerTemplate {
	return s.templates
}

// GetTemplateByID returns a specific template by ID
func (s *TemplateService) GetTemplateByID(id string) (*models.ServerTemplate, error) {
	for _, template := range s.templates {
		if template.ID == id {
			return &template, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// GetTemplatesByCategory returns templates filtered by category
func (s *TemplateService) GetTemplatesByCategory(category string) []models.ServerTemplate {
	var filtered []models.ServerTemplate
	for _, template := range s.templates {
		if template.Category == category {
			filtered = append(filtered, template)
		}
	}
	return filtered
}

// GetPopularTemplates returns only popular templates
func (s *TemplateService) GetPopularTemplates() []models.ServerTemplate {
	var popular []models.ServerTemplate
	for _, template := range s.templates {
		if template.Popular {
			popular = append(popular, template)
		}
	}
	return popular
}

// GetCategories returns all template categories
func (s *TemplateService) GetCategories() []models.TemplateCategory {
	return s.categories
}

// SearchTemplates searches templates by name, description, or tags
func (s *TemplateService) SearchTemplates(query string) []models.ServerTemplate {
	query = strings.ToLower(query)
	var results []models.ServerTemplate

	for _, template := range s.templates {
		// Search in name
		if strings.Contains(strings.ToLower(template.Name), query) {
			results = append(results, template)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(template.Description), query) {
			results = append(results, template)
			continue
		}

		// Search in tags
		for _, tag := range template.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, template)
				break
			}
		}
	}

	return results
}

// ApplyTemplateToServer applies a template configuration to a server's server.properties
func (s *TemplateService) ApplyTemplateToServer(serverID string, templateID string) error {
	template, err := s.GetTemplateByID(templateID)
	if err != nil {
		return err
	}

	serverPath := filepath.Join("servers", serverID)
	propertiesPath := filepath.Join(serverPath, "server.properties")

	// Check if server directory exists
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		return fmt.Errorf("server directory not found: %s", serverID)
	}

	// Read existing properties if they exist
	existingProps := make(map[string]string)
	if data, err := os.ReadFile(propertiesPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				existingProps[parts[0]] = parts[1]
			}
		}
	}

	// Apply template properties
	for key, value := range template.Properties {
		existingProps[key] = fmt.Sprintf("%v", value)
	}

	// Write updated properties
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Server Properties - Template: %s\n", template.Name))
	content.WriteString(fmt.Sprintf("# Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	for key, value := range existingProps {
		content.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	if err := os.WriteFile(propertiesPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("failed to write server.properties: %w", err)
	}

	logger.Info("Template applied to server", map[string]interface{}{
		"server_id":   serverID,
		"template_id": templateID,
		"template":    template.Name,
	})

	return nil
}

// GetTemplateRecommendations returns recommended templates based on criteria
func (s *TemplateService) GetTemplateRecommendations(playerCount int, modded bool) []models.ServerTemplate {
	var recommendations []models.ServerTemplate

	for _, template := range s.templates {
		// Filter by player count (max-players in properties)
		if maxPlayers, ok := template.Properties["max-players"].(float64); ok {
			if int(maxPlayers) < playerCount {
				continue
			}
		}

		// Filter by modded requirement
		if modded && template.Category != "modded" {
			continue
		}

		recommendations = append(recommendations, template)
	}

	// Prioritize popular templates
	var popular, others []models.ServerTemplate
	for _, t := range recommendations {
		if t.Popular {
			popular = append(popular, t)
		} else {
			others = append(others, t)
		}
	}

	return append(popular, others...)
}
