package velocity

// VelocityConfig represents the complete Velocity proxy configuration
type VelocityConfig struct {
	ConfigVersion string                     `toml:"config-version"`
	Bind          string                     `toml:"bind"`
	Motd          string                     `toml:"motd"`
	ShowMaxPlayers int                       `toml:"show-max-players"`
	OnlineMode    bool                       `toml:"online-mode"`
	Servers       map[string]ServerConfig    `toml:"servers"`
	Try           []string                   `toml:"try"`
	ForcedHosts   map[string][]string        `toml:"forced-hosts"`
}

// ServerConfig represents a single backend server configuration
type ServerConfig struct {
	Address string `toml:"-"` // Not directly in TOML, stored as map key -> value
}

// WakeupStatus represents the status of a server wakeup operation
type WakeupStatus struct {
	ServerID  string `json:"server_id"`
	Status    string `json:"status"` // "starting", "running", "failed"
	Message   string `json:"message,omitempty"`
	Port      int    `json:"port,omitempty"`
	Ready     bool   `json:"ready"`
}
