package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"desent/internal/admin"
	"desent/internal/auth"
	"desent/internal/chat"
	"desent/internal/config"
	"desent/internal/db"
	"desent/internal/hls"
	"desent/internal/ingest"
	"desent/internal/setup"
	"desent/internal/update"
)

// Set via -ldflags at build time.
var version = "dev"

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Handle update-self mode: a helper container uses this to recreate the server container.
	if len(os.Args) > 1 && os.Args[1] == "update-self" {
		runUpdateSelf()
		return
	}

	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.ParseLogLevel(cfg.LogLevel),
	})))
	slog.Info("starting desent", "version", version)
	if cfg.JWTSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	// Data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data dir", "path", cfg.DataDir, "err", err)
		os.Exit(1)
	}

	// Database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("database ready", "path", cfg.DBPath)

	// HLS directories — create for ALL quality presets
	for _, q := range ingest.AllQualities {
		dir := filepath.Join(cfg.HLSDir, q.Name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create HLS dir", "path", dir, "err", err)
			os.Exit(1)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Clean stale HLS files from previous run
	hls.CleanAll(cfg.HLSDir)

	// HLS segment cache
	segmentCache, err := hls.NewSegmentCache(cfg.HLSDir, cfg.HLSCacheMB)
	if err != nil {
		slog.Error("failed to create segment cache", "err", err)
		os.Exit(1)
	}
	go segmentCache.Start(ctx)

	// Ingest (FFmpeg subprocess)
	ingestMgr := ingest.NewManager(ingest.Config{
		FFmpegPath: cfg.FFmpegPath,
		RTMPAddr:   cfg.RTMPAddr,
		StreamKey:  cfg.StreamKey,
		HLSDir:     cfg.HLSDir,
		OnStreamEnd: func() {
			hls.CleanAll(cfg.HLSDir)
			segmentCache.Purge()
		},
	})

	// Load persisted qualities from DB
	adminStore := admin.NewStore(database)
	if settings, err := adminStore.GetSettings(ctx); err == nil {
		if v, ok := settings["qualities_enabled"]; ok && v != "" {
			names := strings.Split(v, ",")
			ingestMgr.SetQualities(names)
			slog.Info("loaded persisted qualities", "qualities", names)
		}
		if v, ok := settings["stream_fps"]; ok && v != "" {
			overrides := ingest.ParseFPSOverrides(v)
			ingestMgr.SetFPSOverrides(overrides)
			slog.Info("loaded persisted FPS overrides", "fps", overrides)
		}
		if v, ok := settings["ffmpeg_preset"]; ok && v != "" && ingest.IsValidPreset(v) {
			ingestMgr.SetPreset(v)
			slog.Info("loaded persisted FFmpeg preset", "preset", v, "resolved", ingestMgr.ResolvedPreset())
		}
	}

	go ingestMgr.Run(ctx)

	// HLS segment cleaner
	hls.StartCleaner(ctx, cfg.HLSDir, 30*time.Second, 60*time.Second)

	// Routes
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": version})
	})

	// Auth
	authStore := auth.NewStore(database)
	tokenService := auth.NewTokenService(cfg.JWTSecret)
	authHandler := auth.NewHandler(authStore, tokenService, cfg.BcryptCost)
	authHandler.RegisterRoutes(mux)

	// Setup
	setupStore := setup.NewStore(database)
	setupHandler := setup.NewHandler(setupStore, authStore, tokenService, cfg.BcryptCost, cfg.DataDir)
	setupHandler.RegisterRoutes(mux)

	// Chat
	chatStore := chat.NewStore(database)
	chatHub := chat.NewHub(chatStore, ingestMgr, adminStore)
	go chatHub.Run(ctx)

	chatHandler := chat.NewHandler(chatHub, chatStore, tokenService, cfg.ChatMaxMsgLen, cfg.ChatRateLimitMS)
	chatHandler.RegisterRoutes(mux)

	// HLS streaming
	hlsHandler := hls.NewHandler(cfg.HLSDir, segmentCache)
	hlsHandler.RegisterRoutes(mux)
	mux.HandleFunc("GET /api/stream/status", hlsHandler.StreamStatusHandler(ingestMgr, ingestMgr, ingestMgr, adminStore))

	// Admin API
	adminHandler := admin.NewHandler(adminStore, ingestMgr, cfg.HLSDir, cfg.ServerBandwidthMbps, cfg.DataDir)
	adminMW := func(h http.Handler) http.Handler {
		return auth.RequireAuth(tokenService)(auth.RequireAdmin(h))
	}
	adminHandler.RegisterRoutes(mux, adminMW)

	// Update system
	updater := update.NewUpdater(update.Config{
		CurrentVersion: version,
		GitHubRepo:     cfg.GitHubRepo,
		ServerImage:    cfg.DockerServerImage,
		WebImage:       cfg.DockerWebImage,
		ComposeProject: cfg.ComposeProject,
		DataDir:        cfg.DataDir,
	})
	updateHandler := update.NewHandler(updater, tokenService)
	updateHandler.RegisterRoutes(mux, adminMW)

	// Static files (test player)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("HTTP server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown error", "err", err)
	}
	slog.Info("server stopped")
}

// runUpdateSelf is the helper mode: recreate the server container with a new image.
// Launched by the main server as a short-lived helper container.
func runUpdateSelf() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	fs := flag.NewFlagSet("update-self", flag.ExitOnError)
	containerID := fs.String("container", "", "container ID to recreate")
	newImage := fs.String("image", "", "new image to use")
	containerName := fs.String("name", "", "container name (unused, for logging)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		slog.Error("update-self: parse flags", "err", err)
		os.Exit(1)
	}

	if *containerID == "" || *newImage == "" {
		fmt.Fprintf(os.Stderr, "usage: %s update-self --container ID --image IMAGE [--name NAME]\n", os.Args[0])
		os.Exit(1)
	}

	slog.Info("update-self: starting", "container", *containerID, "image", *newImage, "name", *containerName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	docker := update.NewDockerClient()

	if err := update.RecreateContainer(ctx, docker, *containerID, *newImage); err != nil {
		slog.Error("update-self: recreate failed", "err", err)
		os.Exit(1)
	}

	slog.Info("update-self: server container recreated successfully")
}
