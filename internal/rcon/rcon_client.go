package rcon

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// RCON packet types
const (
	PacketTypeAuth         int32 = 3
	PacketTypeExecCommand  int32 = 2
	PacketTypeResponseAuth int32 = 2
	PacketTypeResponseCmd  int32 = 0
)

// Client represents an RCON client connection
type Client struct {
	conn     net.Conn
	requestID int32
	mu       sync.Mutex
}

// NewClient creates a new RCON client and authenticates
func NewClient(host string, port int, password string) (*Client, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RCON: %w", err)
	}

	client := &Client{
		conn:     conn,
		requestID: 1,
	}

	// Authenticate
	response, err := client.sendPacket(PacketTypeAuth, password)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	if response == "" {
		conn.Close()
		return nil, errors.New("authentication failed: invalid password")
	}

	return client, nil
}

// SendCommand executes a command on the server
func (c *Client) SendCommand(command string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sendPacket(PacketTypeExecCommand, command)
}

// GetPlayerCount returns the number of online players
func (c *Client) GetPlayerCount() (int, error) {
	response, err := c.SendCommand("list")
	if err != nil {
		return 0, err
	}

	// Parse "There are X of a max of Y players online: ..."
	// Simple parsing - can be improved
	var playerCount int
	_, err = fmt.Sscanf(response, "There are %d", &playerCount)
	if err != nil {
		// Try alternative format
		_, err = fmt.Sscanf(response, "%d players online", &playerCount)
		if err != nil {
			return 0, fmt.Errorf("failed to parse player count: %s", response)
		}
	}

	return playerCount, nil
}

// GetPlayers returns a list of online players
func (c *Client) GetPlayers() ([]string, error) {
	response, err := c.SendCommand("list")
	if err != nil {
		return nil, err
	}

	// Parse player names from response
	// Format: "There are X of a max of Y players online: Player1, Player2, ..."
	// or: "There are X/Y players online: Player1, Player2, ..."

	// Try to find the player list after the colon
	parts := strings.Split(response, ":")
	if len(parts) < 2 {
		return []string{}, nil
	}

	playersPart := strings.TrimSpace(parts[1])
	if playersPart == "" {
		return []string{}, nil
	}

	// Split by comma and trim whitespace
	playerNames := strings.Split(playersPart, ",")
	var result []string
	for _, name := range playerNames {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, nil
}

// Close closes the RCON connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// sendPacket sends an RCON packet and returns the response
func (c *Client) sendPacket(packetType int32, payload string) (string, error) {
	c.requestID++

	// Build packet
	packet := buildPacket(c.requestID, packetType, payload)

	// Send packet
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}

	if _, err := c.conn.Write(packet); err != nil {
		return "", fmt.Errorf("failed to write packet: %w", err)
	}

	// Read response
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}

	response, err := readPacket(c.conn)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return response, nil
}

// buildPacket constructs an RCON packet
func buildPacket(requestID, packetType int32, payload string) []byte {
	payloadBytes := []byte(payload)
	length := int32(10 + len(payloadBytes)) // 4 (ID) + 4 (Type) + payload + 2 null bytes

	packet := make([]byte, length+4) // +4 for length field itself

	binary.LittleEndian.PutUint32(packet[0:4], uint32(length))
	binary.LittleEndian.PutUint32(packet[4:8], uint32(requestID))
	binary.LittleEndian.PutUint32(packet[8:12], uint32(packetType))
	copy(packet[12:], payloadBytes)
	// Last 2 bytes are null (padding)

	return packet
}

// readPacket reads an RCON response packet
func readPacket(conn net.Conn) (string, error) {
	// Read packet length
	lengthBuf := make([]byte, 4)
	if _, err := conn.Read(lengthBuf); err != nil {
		return "", err
	}

	length := binary.LittleEndian.Uint32(lengthBuf)

	// Read packet body
	body := make([]byte, length)
	if _, err := conn.Read(body); err != nil {
		return "", err
	}

	// Skip request ID (4 bytes) and packet type (4 bytes)
	// Payload starts at byte 8
	payload := body[8 : len(body)-2] // -2 to remove trailing null bytes

	return string(payload), nil
}

// TryConnect attempts to connect to RCON (for checking if server is ready)
func TryConnect(host string, port int, password string, maxAttempts int) (*Client, error) {
	var lastErr error

	for i := 0; i < maxAttempts; i++ {
		client, err := NewClient(host, port, password)
		if err == nil {
			return client, nil
		}

		lastErr = err
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxAttempts, lastErr)
}
