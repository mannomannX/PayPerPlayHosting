package docker

import (
	"fmt"

	"github.com/payperplay/hosting/internal/models"
)

// ContainerBuilder provides methods to build Docker container configuration from Server model
// This enables both local (via docker client) and remote (via SSH) container creation

// BuildContainerEnv builds environment variables from a MinecraftServer model
// These env vars are compatible with itzg/minecraft-server Docker image
func BuildContainerEnv(server *models.MinecraftServer) []string {
	env := []string{
		"EULA=TRUE",
		fmt.Sprintf("TYPE=%s", getServerTypeEnv(string(server.ServerType))),
		fmt.Sprintf("VERSION=%s", server.MinecraftVersion),
		fmt.Sprintf("MEMORY=%dM", server.RAMMb),
		fmt.Sprintf("MAX_PLAYERS=%d", server.MaxPlayers),
		"ONLINE_MODE=TRUE",
		"SERVER_NAME=PayPerPlay Server",

		// Enable RCON for monitoring
		"ENABLE_RCON=true",
		"RCON_PASSWORD=minecraft",
		"RCON_PORT=25575",

		// === Phase 1 - Gameplay Settings ===
		fmt.Sprintf("MODE=%s", server.Gamemode),
		fmt.Sprintf("DIFFICULTY=%s", server.Difficulty),
		fmt.Sprintf("PVP=%t", server.PVP),
		fmt.Sprintf("ENABLE_COMMAND_BLOCK=%t", server.EnableCommandBlock),

		// === Phase 2 - Performance Settings ===
		fmt.Sprintf("VIEW_DISTANCE=%d", server.ViewDistance),
		fmt.Sprintf("SIMULATION_DISTANCE=%d", server.SimulationDistance),

		// === Phase 2 - World Generation Settings ===
		fmt.Sprintf("ALLOW_NETHER=%t", server.AllowNether),
		fmt.Sprintf("GENERATE_STRUCTURES=%t", server.GenerateStructures),
		fmt.Sprintf("LEVEL_TYPE=%s", server.WorldType),
		fmt.Sprintf("ENABLE_BONUS_CHEST=%t", server.BonusChest),
		fmt.Sprintf("MAX_WORLD_SIZE=%d", server.MaxWorldSize),

		// === Phase 2 - Spawn Settings ===
		fmt.Sprintf("SPAWN_PROTECTION=%d", server.SpawnProtection),
		fmt.Sprintf("SPAWN_ANIMALS=%t", server.SpawnAnimals),
		fmt.Sprintf("SPAWN_MONSTERS=%t", server.SpawnMonsters),
		fmt.Sprintf("SPAWN_NPCS=%t", server.SpawnNPCs),

		// === Phase 2 - Network & Performance Settings ===
		fmt.Sprintf("MAX_TICK_TIME=%d", server.MaxTickTime),
		fmt.Sprintf("NETWORK_COMPRESSION_THRESHOLD=%d", server.NetworkCompressionThreshold),

		// === Phase 4 - Server Description ===
		fmt.Sprintf("MOTD=%s", server.MOTD),
	}

	// Add SEED only if provided (empty = random)
	if server.LevelSeed != "" {
		env = append(env, fmt.Sprintf("SEED=%s", server.LevelSeed))
	}

	return env
}

// BuildPortBindings builds port mapping for Docker container
// Returns map of internal port -> host port (e.g., "25565/tcp" -> 25577)
func BuildPortBindings(hostPort int) map[string]int {
	return map[string]int{
		"25565/tcp": hostPort, // Minecraft server port
		// Note: RCON port (25575) is NOT mapped - we use docker exec for commands
	}
}

// BuildVolumeBinds builds volume bindings for Docker container
// Returns array of bind mounts (e.g., "/path/on/host:/data")
func BuildVolumeBinds(serverID string, hostServersBasePath string) []string {
	return []string{
		fmt.Sprintf("%s/%s:/data", hostServersBasePath, serverID),
	}
}

// GetDockerImageName returns the Docker image name for a Minecraft server
func GetDockerImageName(serverType string) string {
	// Currently we use itzg/minecraft-server for all server types
	// In the future, we could have different images for different types
	return "itzg/minecraft-server:latest"
}

// getServerTypeEnv converts our internal server type to itzg/minecraft-server TYPE env var
func getServerTypeEnv(serverType string) string {
	// Map our server types to itzg/minecraft-server TYPE values
	switch serverType {
	case "vanilla":
		return "VANILLA"
	case "paper":
		return "PAPER"
	case "spigot":
		return "SPIGOT"
	case "forge":
		return "FORGE"
	case "fabric":
		return "FABRIC"
	case "purpur":
		return "PURPUR"
	default:
		return "PAPER" // Default to Paper if unknown
	}
}
