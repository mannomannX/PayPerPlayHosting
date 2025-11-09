package monitoring

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorcon/rcon"
	"github.com/payperplay/hosting/pkg/logger"
)

// RCONClient wraps the Minecraft RCON client
type RCONClient struct {
	host     string
	port     int
	password string
}

// NewRCONClient creates a new RCON client
func NewRCONClient(host string, port int, password string) *RCONClient {
	return &RCONClient{
		host:     host,
		port:     port,
		password: password,
	}
}

// GetTPS retrieves the server TPS (Ticks Per Second)
// For Paper/Spigot: uses "/tps" command
// For Vanilla: returns -1 (no TPS command available)
func (r *RCONClient) GetTPS() (float64, error) {
	conn, err := rcon.Dial(fmt.Sprintf("%s:%d", r.host, r.port), r.password)
	if err != nil {
		return -1, fmt.Errorf("RCON connection failed: %w", err)
	}
	defer conn.Close()

	// Try Paper/Spigot TPS command
	response, err := conn.Execute("tps")
	if err != nil {
		return -1, fmt.Errorf("TPS command failed: %w", err)
	}

	// Parse TPS from response
	// Example: "§aTPS from last 1m, 5m, 15m: §r§a20.0, §r§a20.0, §r§a20.0"
	tps := parseTPS(response)
	if tps > 0 {
		return tps, nil
	}

	// Fallback: return -1 for vanilla servers (no TPS command)
	return -1, nil
}

// GetPlayerCount retrieves the current player count
func (r *RCONClient) GetPlayerCount() (int, int, error) {
	conn, err := rcon.Dial(fmt.Sprintf("%s:%d", r.host, r.port), r.password)
	if err != nil {
		return 0, 0, fmt.Errorf("RCON connection failed: %w", err)
	}
	defer conn.Close()

	// Use "list" command to get player count
	response, err := conn.Execute("list")
	if err != nil {
		return 0, 0, fmt.Errorf("list command failed: %w", err)
	}

	// Parse player count from response
	// Example: "There are 3 of a max of 20 players online:"
	current, max := parsePlayerCount(response)
	return current, max, nil
}

// parseTPS extracts TPS value from command response
func parseTPS(response string) float64 {
	// Remove color codes (§x)
	cleanResponse := regexp.MustCompile(`§.`).ReplaceAllString(response, "")

	// Try to find TPS pattern: "20.0" or similar
	// Paper format: "TPS from last 1m, 5m, 15m: 20.0, 20.0, 20.0"
	if strings.Contains(strings.ToLower(cleanResponse), "tps") {
		// Extract first decimal number after "TPS"
		re := regexp.MustCompile(`TPS.*?([0-9]+\.?[0-9]*)`)
		matches := re.FindStringSubmatch(cleanResponse)
		if len(matches) > 1 {
			if tps, err := strconv.ParseFloat(matches[1], 64); err == nil {
				return tps
			}
		}

		// Alternative: just find first decimal number
		re = regexp.MustCompile(`([0-9]+\.[0-9]+)`)
		matches = re.FindStringSubmatch(cleanResponse)
		if len(matches) > 0 {
			if tps, err := strconv.ParseFloat(matches[0], 64); err == nil {
				return tps
			}
		}
	}

	return -1
}

// parsePlayerCount extracts player count from "list" command response
func parsePlayerCount(response string) (current int, max int) {
	// Remove color codes
	cleanResponse := regexp.MustCompile(`§.`).ReplaceAllString(response, "")

	// Example: "There are 3 of a max of 20 players online:"
	re := regexp.MustCompile(`There are (\d+) of a max (?:of )?(\d+) players`)
	matches := re.FindStringSubmatch(cleanResponse)

	if len(matches) == 3 {
		current, _ = strconv.Atoi(matches[1])
		max, _ = strconv.Atoi(matches[2])
		return current, max
	}

	// Alternative format: "There are 3/20 players online:"
	re = regexp.MustCompile(`There are (\d+)/(\d+) players`)
	matches = re.FindStringSubmatch(cleanResponse)

	if len(matches) == 3 {
		current, _ = strconv.Atoi(matches[1])
		max, _ = strconv.Atoi(matches[2])
		return current, max
	}

	return 0, 0
}

// TestConnection tests if RCON connection works
func (r *RCONClient) TestConnection() error {
	conn, err := rcon.Dial(fmt.Sprintf("%s:%d", r.host, r.port), r.password)
	if err != nil {
		return fmt.Errorf("RCON connection failed: %w", err)
	}
	defer conn.Close()

	// Simple command to verify connection
	_, err = conn.Execute("list")
	if err != nil {
		return fmt.Errorf("RCON command failed: %w", err)
	}

	return nil
}

// SafeGetPlayerCount safely retrieves player count with error handling
func SafeGetPlayerCount(host string, port int, password string) (current int, max int) {
	client := NewRCONClient(host, port, password)

	current, max, err := client.GetPlayerCount()
	if err != nil {
		logger.Debug("Failed to get player count via RCON", map[string]interface{}{
			"host":  host,
			"port":  port,
			"error": err.Error(),
		})
		return 0, 0
	}

	return current, max
}

// SafeGetTPS safely retrieves TPS with error handling
func SafeGetTPS(host string, port int, password string) float64 {
	client := NewRCONClient(host, port, password)

	tps, err := client.GetTPS()
	if err != nil {
		logger.Debug("Failed to get TPS via RCON", map[string]interface{}{
			"host":  host,
			"port":  port,
			"error": err.Error(),
		})
		return -1
	}

	return tps
}

// ExecuteCommand executes an arbitrary RCON command
func ExecuteCommand(host string, port int, password string, command string) (string, error) {
	conn, err := rcon.Dial(fmt.Sprintf("%s:%d", host, port), password, rcon.SetDialTimeout(5*time.Second))
	if err != nil {
		return "", fmt.Errorf("RCON connection failed: %w", err)
	}
	defer conn.Close()

	response, err := conn.Execute(command)
	if err != nil {
		return "", fmt.Errorf("RCON command failed: %w", err)
	}

	return response, nil
}
