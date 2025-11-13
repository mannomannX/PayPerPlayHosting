package com.payperplay.velocity;

import com.google.inject.Inject;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.player.ServerPreConnectEvent;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.event.proxy.ProxyShutdownEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.proxy.server.RegisteredServer;
import com.velocitypowered.api.proxy.server.ServerInfo;
import org.slf4j.Logger;
import io.javalin.Javalin;
import io.javalin.http.Context;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.format.NamedTextColor;

import java.net.InetSocketAddress;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.stream.Collectors;

/**
 * Velocity Remote API Plugin
 *
 * Provides HTTP REST API for dynamic server registration/unregistration.
 * This allows the PayPerPlay Control Plane to manage Minecraft servers without direct Velocity config access.
 *
 * Endpoints:
 * - POST   /api/servers         - Register a new backend server
 * - DELETE /api/servers/{name}  - Unregister a backend server
 * - GET    /api/servers         - List all registered servers
 * - GET    /api/players/{server} - Get player count for a specific server
 * - GET    /health               - Health check endpoint
 */
@Plugin(
    id = "velocity-remote-api",
    name = "VelocityRemoteAPI",
    version = "1.0.0",
    description = "HTTP API for dynamic server registration",
    authors = {"PayPerPlay"}
)
public class RemoteAPI {

    private final ProxyServer server;
    private final Logger logger;
    private Javalin app;

    @Inject
    public RemoteAPI(ProxyServer server, Logger logger) {
        this.server = server;
        this.logger = logger;
    }

    @Subscribe
    public void onProxyInitialization(ProxyInitializeEvent event) {
        logger.info("Starting VelocityRemoteAPI HTTP server on port 8080...");

        app = Javalin.create(config -> {
            config.showJavalinBanner = false;
        }).start(8080);

        // Register endpoints
        app.post("/api/servers", this::registerServer);
        app.delete("/api/servers/{name}", this::unregisterServer);
        app.get("/api/servers", this::listServers);
        app.get("/api/players/{server}", this::getPlayerCount);
        app.get("/health", this::healthCheck);

        logger.info("VelocityRemoteAPI initialized successfully on port 8080");
    }

    @Subscribe
    public void onProxyShutdown(ProxyShutdownEvent event) {
        if (app != null) {
            app.stop();
            logger.info("VelocityRemoteAPI HTTP server stopped");
        }
    }

    /**
     * POST /api/servers
     * Body: {"name": "server-name", "address": "host:port"}
     *
     * Example: {"name": "survival-1", "address": "91.98.202.235:25566"}
     */
    @SuppressWarnings("unchecked")
    private void registerServer(Context ctx) {
        try {
            Map<String, String> body = ctx.bodyAsClass(Map.class);
            String name = body.get("name");
            String address = body.get("address");

            if (name == null || address == null) {
                ctx.status(400).json(Map.of("error", "Missing 'name' or 'address' field"));
                return;
            }

            // Parse address (format: "host:port")
            String[] parts = address.split(":");
            if (parts.length != 2) {
                ctx.status(400).json(Map.of("error", "Invalid address format, expected 'host:port'"));
                return;
            }

            String host = parts[0];
            int port;
            try {
                port = Integer.parseInt(parts[1]);
            } catch (NumberFormatException e) {
                ctx.status(400).json(Map.of("error", "Invalid port number"));
                return;
            }

            // Create ServerInfo and register
            ServerInfo serverInfo = new ServerInfo(name, new InetSocketAddress(host, port));
            server.registerServer(serverInfo);

            logger.info("Registered server: {} at {}", name, address);
            ctx.status(200).json(Map.of(
                "status", "ok",
                "message", "Server registered successfully",
                "name", name,
                "address", address
            ));

        } catch (Exception e) {
            logger.error("Failed to register server", e);
            ctx.status(500).json(Map.of("error", "Internal server error: " + e.getMessage()));
        }
    }

    /**
     * DELETE /api/servers/{name}
     *
     * Example: DELETE /api/servers/survival-1
     */
    private void unregisterServer(Context ctx) {
        try {
            String name = ctx.pathParam("name");

            RegisteredServer registeredServer = server.getServer(name).orElse(null);
            if (registeredServer == null) {
                ctx.status(404).json(Map.of("error", "Server not found"));
                return;
            }

            server.unregisterServer(registeredServer.getServerInfo());
            logger.info("Unregistered server: {}", name);

            ctx.status(200).json(Map.of(
                "status", "ok",
                "message", "Server unregistered successfully",
                "name", name
            ));

        } catch (Exception e) {
            logger.error("Failed to unregister server", e);
            ctx.status(500).json(Map.of("error", "Internal server error: " + e.getMessage()));
        }
    }

    /**
     * GET /api/servers
     *
     * Returns list of all registered backend servers with player counts
     */
    private void listServers(Context ctx) {
        try {
            var servers = server.getAllServers().stream()
                .map(registeredServer -> {
                    Map<String, Object> serverData = new HashMap<>();
                    ServerInfo info = registeredServer.getServerInfo();
                    serverData.put("name", info.getName());
                    serverData.put("address", info.getAddress().getHostString() + ":" + info.getAddress().getPort());
                    serverData.put("players", registeredServer.getPlayersConnected().size());
                    return serverData;
                })
                .collect(Collectors.toList());

            ctx.status(200).json(Map.of(
                "status", "ok",
                "count", servers.size(),
                "servers", servers
            ));

        } catch (Exception e) {
            logger.error("Failed to list servers", e);
            ctx.status(500).json(Map.of("error", "Internal server error: " + e.getMessage()));
        }
    }

    /**
     * GET /api/players/{server}
     *
     * Returns player count for a specific server
     */
    private void getPlayerCount(Context ctx) {
        try {
            String serverName = ctx.pathParam("server");
            RegisteredServer registeredServer = server.getServer(serverName).orElse(null);

            if (registeredServer == null) {
                ctx.status(404).json(Map.of("error", "Server not found"));
                return;
            }

            int playerCount = registeredServer.getPlayersConnected().size();
            ctx.status(200).json(Map.of(
                "status", "ok",
                "server", serverName,
                "players", playerCount
            ));

        } catch (Exception e) {
            logger.error("Failed to get player count", e);
            ctx.status(500).json(Map.of("error", "Internal server error: " + e.getMessage()));
        }
    }

    /**
     * GET /health
     *
     * Health check endpoint
     */
    private void healthCheck(Context ctx) {
        ctx.status(200).json(Map.of(
            "status", "ok",
            "version", "1.0.0",
            "servers_count", server.getAllServers().size(),
            "players_online", server.getPlayerCount()
        ));
    }

    /**
     * Handle player initial connection - route to first available server
     * This is triggered when a player first connects to the proxy (not when switching servers)
     */
    @Subscribe
    public void onPlayerConnect(ServerPreConnectEvent event) {
        // Only handle initial connections (when player joins proxy)
        // ServerPreConnectEvent fires for both initial connections and server switches
        Optional<RegisteredServer> previousServer = event.getPreviousServer();

        // If player is switching between servers, don't interfere
        if (previousServer.isPresent()) {
            logger.debug("Player {} switching servers, not routing", event.getPlayer().getUsername());
            return;
        }

        // Get all registered servers
        List<RegisteredServer> availableServers = server.getAllServers().stream()
            .filter(s -> !s.getPlayersConnected().isEmpty() || true) // Include all servers for now
            .collect(Collectors.toList());

        if (availableServers.isEmpty()) {
            logger.warn("Player {} tried to connect but no servers are registered", event.getPlayer().getUsername());
            event.setResult(ServerPreConnectEvent.ServerResult.denied());
            event.getPlayer().disconnect(
                Component.text("No servers available. Please try again later.")
                    .color(NamedTextColor.RED)
            );
            return;
        }

        // Route to first available server
        RegisteredServer targetServer = availableServers.get(0);
        event.setResult(ServerPreConnectEvent.ServerResult.allowed(targetServer));

        logger.info("Routing player {} to server {}",
            event.getPlayer().getUsername(),
            targetServer.getServerInfo().getName()
        );
    }
}
