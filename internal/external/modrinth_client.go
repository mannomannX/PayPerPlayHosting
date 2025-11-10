package external

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

const (
	ModrinthAPIBase = "https://api.modrinth.com/v2"
	UserAgent       = "PayPerPlay/1.0 (hosting@payperplay.com)"
)

// ModrinthClient handles communication with Modrinth API
type ModrinthClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewModrinthClient creates a new Modrinth API client
func NewModrinthClient() *ModrinthClient {
	return &ModrinthClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: ModrinthAPIBase,
	}
}

// === Modrinth API Response Structures ===

// ModrinthSearchResponse represents search results from Modrinth
type ModrinthSearchResponse struct {
	Hits   []ModrinthProject `json:"hits"`
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
	Total  int               `json:"total_hits"`
}

// ModrinthProject represents a plugin/mod project on Modrinth
type ModrinthProject struct {
	ProjectID         string   `json:"project_id"`
	Slug              string   `json:"slug"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Categories        []string `json:"categories"`
	DisplayCategories []string `json:"display_categories"`
	Author            string   `json:"author"`
	IconURL           string   `json:"icon_url"`
	Downloads         int      `json:"downloads"`
	ProjectType       string   `json:"project_type"` // "mod", "plugin", "modpack"

	// Additional fields available when querying single project
	Body           string   `json:"body,omitempty"`
	Issues         string   `json:"issues_url,omitempty"`
	Source         string   `json:"source_url,omitempty"`
	Wiki           string   `json:"wiki_url,omitempty"`
	Discord        string   `json:"discord_url,omitempty"`
}

// ModrinthVersion represents a specific version of a plugin
type ModrinthVersion struct {
	ID              string               `json:"id"`
	ProjectID       string               `json:"project_id"`
	VersionNumber   string               `json:"version_number"`
	VersionType     string               `json:"version_type"` // "release", "beta", "alpha"
	Changelog       string               `json:"changelog"`
	Dependencies    []ModrinthDependency `json:"dependencies"`
	GameVersions    []string             `json:"game_versions"`    // Minecraft versions
	Loaders         []string             `json:"loaders"`          // "paper", "spigot", "fabric", etc.
	Files           []ModrinthFile       `json:"files"`
	DatePublished   time.Time            `json:"date_published"`
	Downloads       int                  `json:"downloads"`
	Featured        bool                 `json:"featured"`
}

// ModrinthDependency represents a dependency of a plugin version
type ModrinthDependency struct {
	VersionID      string `json:"version_id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	FileName       string `json:"file_name,omitempty"`
	DependencyType string `json:"dependency_type"` // "required", "optional", "incompatible"
}

// ModrinthFile represents a downloadable file
type ModrinthFile struct {
	Hashes   ModrinthHashes `json:"hashes"`
	URL      string         `json:"url"`
	Filename string         `json:"filename"`
	Primary  bool           `json:"primary"`
	Size     int64          `json:"size"`
	FileType string         `json:"file_type"` // "required-resource-pack", "optional-resource-pack", etc.
}

// ModrinthHashes contains file integrity hashes
type ModrinthHashes struct {
	SHA1   string `json:"sha1"`
	SHA512 string `json:"sha512"`
}

// === API Methods ===

// SearchPlugins searches for Paper/Spigot plugins on Modrinth
func (c *ModrinthClient) SearchPlugins(query string, limit int, offset int) (*ModrinthSearchResponse, error) {
	// Build facets for Paper/Spigot plugins only
	facets := `[["project_type:plugin"],["categories:paper"]]`

	params := url.Values{}
	if query != "" {
		params.Add("query", query)
	}
	params.Add("facets", facets)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	searchURL := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

	resp, err := c.doRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp ModrinthSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &searchResp, nil
}

// GetProject retrieves a single project by slug or ID
func (c *ModrinthClient) GetProject(slugOrID string) (*ModrinthProject, error) {
	projectURL := fmt.Sprintf("%s/project/%s", c.baseURL, slugOrID)

	resp, err := c.doRequest("GET", projectURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var project ModrinthProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project response: %w", err)
	}

	return &project, nil
}

// GetProjectVersions retrieves all versions for a project
func (c *ModrinthClient) GetProjectVersions(projectID string) ([]ModrinthVersion, error) {
	versionsURL := fmt.Sprintf("%s/project/%s/version", c.baseURL, projectID)

	resp, err := c.doRequest("GET", versionsURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var versions []ModrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode versions response: %w", err)
	}

	return versions, nil
}

// GetVersion retrieves a specific version by ID
func (c *ModrinthClient) GetVersion(versionID string) (*ModrinthVersion, error) {
	versionURL := fmt.Sprintf("%s/version/%s", c.baseURL, versionID)

	resp, err := c.doRequest("GET", versionURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var version ModrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return nil, fmt.Errorf("failed to decode version response: %w", err)
	}

	return &version, nil
}

// GetPopularPlugins retrieves the most popular Paper plugins
func (c *ModrinthClient) GetPopularPlugins(limit int) (*ModrinthSearchResponse, error) {
	facets := `[["project_type:plugin"],["categories:paper"]]`

	params := url.Values{}
	params.Add("facets", facets)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("index", "downloads") // Sort by downloads

	searchURL := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

	resp, err := c.doRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp ModrinthSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &searchResp, nil
}

// === Helper Methods ===

// doRequest performs an HTTP request with proper headers
func (c *ModrinthClient) doRequest(method, url string, body interface{}) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	logger.Debug("Modrinth API request", map[string]interface{}{
		"method": method,
		"url":    url,
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

// IsVersionCompatible checks if a version is compatible with server specs
func IsVersionCompatible(version *ModrinthVersion, minecraftVersion string, serverType string) bool {
	// Check Minecraft version compatibility
	mcCompatible := false
	for _, v := range version.GameVersions {
		if v == minecraftVersion {
			mcCompatible = true
			break
		}
	}
	if !mcCompatible {
		return false
	}

	// Check server type (loader) compatibility
	loaderCompatible := false
	for _, loader := range version.Loaders {
		// Paper is compatible with Paper, Spigot, and Bukkit loaders
		if loader == serverType || loader == "paper" || loader == "spigot" || loader == "bukkit" {
			loaderCompatible = true
			break
		}
	}

	return loaderCompatible
}
