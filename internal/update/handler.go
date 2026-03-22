package update

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"desent/internal/auth"
)

// Handler exposes HTTP endpoints for the update system.
type Handler struct {
	updater      *Updater
	tokenService *auth.TokenService
}

// NewHandler creates a new update Handler.
func NewHandler(updater *Updater, tokenService *auth.TokenService) *Handler {
	return &Handler{
		updater:      updater,
		tokenService: tokenService,
	}
}

// RegisterRoutes registers update endpoints.
// Check and apply use standard admin middleware (Bearer token).
// Progress uses query-param auth (SSE/EventSource can't set headers).
func (h *Handler) RegisterRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/admin/update/check", mw(http.HandlerFunc(h.checkUpdate)))
	mux.Handle("POST /api/admin/update/apply", mw(http.HandlerFunc(h.applyUpdate)))
	mux.HandleFunc("GET /api/admin/update/progress", h.streamProgress)
}

func (h *Handler) checkUpdate(w http.ResponseWriter, r *http.Request) {
	result, err := h.updater.CheckForUpdate(r.Context())
	if err != nil {
		slog.Error("update: check failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to check for update: %v", err)})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) applyUpdate(w http.ResponseWriter, r *http.Request) {
	if !SocketAvailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Docker socket not available. Mount /var/run/docker.sock into the server container to enable updates.",
		})
		return
	}

	if h.updater.IsUpdating() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "update already in progress"})
		return
	}

	if err := h.updater.Apply(r.Context()); err != nil {
		slog.Error("update: apply failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "update started"})
}

// streamProgress serves an SSE stream of update progress.
// Auth via ?token= query param (same pattern as WebSocket chat).
func (h *Handler) streamProgress(w http.ResponseWriter, r *http.Request) {
	// Auth: token in query param.
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
		return
	}
	claims, err := h.tokenService.Validate(token)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := h.updater.Subscribe()
	defer h.updater.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(p)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
