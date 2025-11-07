package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FeatureMapping represents a feature and its minimum version
type FeatureMapping struct {
	Feature    string `json:"feature"`
	MinVersion string `json:"min_version"`
	Source     string `json:"source"`
	Notes      string `json:"notes"`
}

// VersionFeaturesData contains all feature mappings
type VersionFeaturesData struct {
	LastUpdated string           `json:"last_updated"`
	Features    []FeatureMapping `json:"features"`
}

func main() {
	fmt.Println("PayPerPlay - Version Features Updater")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("This tool helps maintain the version-features.js file.")
	fmt.Println("It allows you to:")
	fmt.Println("  1. Add new feature mappings")
	fmt.Println("  2. Update existing mappings")
	fmt.Println("  3. Export data to JSON for backup")
	fmt.Println()

	// Check if running from project root
	if _, err := os.Stat("web/static/js"); err != nil {
		fmt.Println("Error: Please run this tool from the project root directory")
		fmt.Println("Current directory:", getCurrentDir())
		return
	}

	// Main menu
	for {
		fmt.Println("\nOptions:")
		fmt.Println("  1. Add new feature mapping")
		fmt.Println("  2. View current mappings")
		fmt.Println("  3. Export to JSON")
		fmt.Println("  4. Check Minecraft Wiki for feature info")
		fmt.Println("  5. Exit")
		fmt.Print("\nSelect option: ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			addFeatureMapping()
		case 2:
			viewMappings()
		case 3:
			exportToJSON()
		case 4:
			checkMinecraftWiki()
		case 5:
			fmt.Println("\nExiting...")
			return
		default:
			fmt.Println("\nInvalid option, please try again")
		}
	}
}

func addFeatureMapping() {
	fmt.Println("\n--- Add New Feature Mapping ---")
	fmt.Print("Feature name (e.g., 'gamemode_spectator'): ")
	var feature string
	fmt.Scanln(&feature)

	fmt.Print("Minimum version (e.g., '1.8.0'): ")
	var minVersion string
	fmt.Scanln(&minVersion)

	fmt.Print("Source (e.g., 'Minecraft Wiki'): ")
	var source string
	fmt.Scanln(&source)

	fmt.Print("Notes (optional): ")
	var notes string
	fmt.Scanln(&notes)

	mapping := FeatureMapping{
		Feature:    feature,
		MinVersion: minVersion,
		Source:     source,
		Notes:      notes,
	}

	fmt.Println("\n✅ Feature mapping added:")
	fmt.Printf("   Feature: %s\n", mapping.Feature)
	fmt.Printf("   Min Version: %s\n", mapping.MinVersion)
	fmt.Printf("   Source: %s\n", mapping.Source)
	fmt.Printf("   Notes: %s\n", mapping.Notes)
	fmt.Println("\n⚠️  To apply this change, manually update web/static/js/version-features.js")
	fmt.Println("   Add this line to FEATURE_MIN_VERSIONS:")
	fmt.Printf("   '%s': '%s',  // %s\n", mapping.Feature, mapping.MinVersion, mapping.Notes)
}

func viewMappings() {
	fmt.Println("\n--- Current Feature Mappings ---")
	fmt.Println()
	fmt.Println("From web/static/js/version-features.js:")
	fmt.Println()

	// Read the JS file
	content, err := os.ReadFile("web/static/js/version-features.js")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Extract FEATURE_MIN_VERSIONS section
	lines := strings.Split(string(content), "\n")
	inFeatureSection := false

	for _, line := range lines {
		if strings.Contains(line, "const FEATURE_MIN_VERSIONS = {") {
			inFeatureSection = true
			continue
		}

		if inFeatureSection && strings.Contains(line, "};") {
			break
		}

		if inFeatureSection && strings.TrimSpace(line) != "" {
			// Print the feature line
			fmt.Println("  ", line)
		}
	}

	fmt.Println()
}

func exportToJSON() {
	fmt.Println("\n--- Export to JSON ---")

	// Read current features from JS file
	content, err := os.ReadFile("web/static/js/version-features.js")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	// Parse features
	lines := strings.Split(string(content), "\n")
	inFeatureSection := false
	features := []FeatureMapping{}

	for _, line := range lines {
		if strings.Contains(line, "const FEATURE_MIN_VERSIONS = {") {
			inFeatureSection = true
			continue
		}

		if inFeatureSection && strings.Contains(line, "};") {
			break
		}

		if inFeatureSection && strings.Contains(line, ":") {
			// Parse feature line: 'feature_name': '1.0.0',  // comment
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				feature := strings.Trim(strings.TrimSpace(parts[0]), "'\"")
				versionAndComment := parts[1]

				// Extract version
				versionParts := strings.Split(versionAndComment, ",")
				version := strings.Trim(strings.TrimSpace(versionParts[0]), "'\"")

				// Extract comment if exists
				notes := ""
				if strings.Contains(versionAndComment, "//") {
					commentParts := strings.Split(versionAndComment, "//")
					if len(commentParts) > 1 {
						notes = strings.TrimSpace(commentParts[1])
					}
				}

				features = append(features, FeatureMapping{
					Feature:    feature,
					MinVersion: version,
					Source:     "version-features.js",
					Notes:      notes,
				})
			}
		}
	}

	data := VersionFeaturesData{
		LastUpdated: time.Now().Format(time.RFC3339),
		Features:    features,
	}

	// Write to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error creating JSON: %v\n", err)
		return
	}

	filename := fmt.Sprintf("version-features-backup-%s.json", time.Now().Format("2006-01-02"))
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("✅ Exported %d features to %s\n", len(features), filename)
}

func checkMinecraftWiki() {
	fmt.Println("\n--- Check Minecraft Wiki ---")
	fmt.Print("Enter feature name to search (e.g., 'Spectator', 'Command Block'): ")
	var feature string
	fmt.Scanln(&feature)

	url := fmt.Sprintf("https://minecraft.wiki/w/%s", strings.ReplaceAll(feature, " ", "_"))
	fmt.Printf("\n🔍 Checking: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching page: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("⚠️  Page not found (Status: %d)\n", resp.StatusCode)
		fmt.Println("Try searching manually: https://minecraft.wiki")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	// Simple check for version mentions
	content := string(body)
	fmt.Println("\n✅ Page found! Here are some version mentions:")
	fmt.Println("(Manual review recommended)")
	fmt.Println()

	// Look for common version patterns
	versions := []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6", "1.7", "1.8", "1.9", "1.10", "1.11", "1.12", "1.13", "1.14", "1.15", "1.16", "1.17", "1.18", "1.19", "1.20", "1.21"}

	for _, version := range versions {
		if strings.Contains(content, fmt.Sprintf("Java Edition %s", version)) {
			fmt.Printf("  - Found mention of Java Edition %s\n", version)
		}
	}

	fmt.Printf("\n🌐 Full URL: %s\n", url)
	fmt.Println("Please review the page manually for accurate version information")
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Base(dir)
}
