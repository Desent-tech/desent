package clip

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"desent/internal/auth"
)

// SessionProvider fetches session info to find VOD paths.
type SessionProvider interface {
	GetVODPath(ctx context.Context, sessionID int64) (string, error)
}

type Handler struct {
	store       *Store
	clipsDir    string
	ffmpegPath  string
	maxDuration int
	sessions    SessionProvider
}

func NewHandler(store *Store, clipsDir, ffmpegPath string, maxDuration int, sessions SessionProvider) *Handler {
	return &Handler{
		store:       store,
		clipsDir:    clipsDir,
		ffmpegPath:  ffmpegPath,
		maxDuration: maxDuration,
		sessions:    sessions,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("POST /api/clips", authMW(http.HandlerFunc(h.createClip)))
	mux.HandleFunc("GET /api/clips", h.listClips)
	mux.HandleFunc("GET /api/clips/recent", h.listRecentClips)
	mux.HandleFunc("GET /clips/{filename}", h.serveClip)
}

func (h *Handler) RegisterAdminRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("DELETE /api/admin/clips/{id}", mw(http.HandlerFunc(h.deleteClip)))
}

type createClipRequest struct {
	SessionID int64  `json:"session_id"`
	StartTime int    `json:"start_time"`
	Duration  int    `json:"duration"`
	Title     string `json:"title"`
}

func (h *Handler) createClip(w http.ResponseWriter, r *http.Request) {
	var req createClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.SessionID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}
	if req.Duration < 15 || req.Duration > h.maxDuration {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("duration must be 15-%d seconds", h.maxDuration)})
		return
	}
	if req.StartTime < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "start_time must be >= 0"})
		return
	}

	// Get VOD path
	vodPath, err := h.sessions.GetVODPath(r.Context(), req.SessionID)
	if err != nil || vodPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session has no VOD recording"})
		return
	}

	if _, err := os.Stat(vodPath); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "VOD file not found"})
		return
	}

	claims := auth.ClaimsFromContext(r.Context())

	// Generate unique filename
	filename := fmt.Sprintf("clip_%d_%d.mp4", req.SessionID, time.Now().UnixMilli())
	clipPath := filepath.Join(h.clipsDir, filename)

	if req.Title == "" {
		req.Title = fmt.Sprintf("Clip from stream #%d", req.SessionID)
	}

	// Create DB record first
	clipID, err := h.store.CreateClip(r.Context(), req.SessionID, claims.UserID, req.Title, filename, req.StartTime, req.Duration)
	if err != nil {
		slog.Error("clip: create record", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Extract clip async
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, h.ffmpegPath,
			"-y",
			"-ss", strconv.Itoa(req.StartTime),
			"-t", strconv.Itoa(req.Duration),
			"-i", vodPath,
			"-c", "copy",
			"-movflags", "+faststart",
			clipPath,
		)

		if err := cmd.Run(); err != nil {
			slog.Error("clip: ffmpeg extract", "err", err, "clip_id", clipID)
			// Clean up DB record on failure
			h.store.DeleteClip(context.Background(), clipID)
			return
		}

		slog.Info("clip: created", "clip_id", clipID, "filename", filename)
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":       clipID,
		"filename": filename,
		"status":   "processing",
	})
}

func (h *Handler) listClips(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.URL.Query().Get("session_id")
	if sessionIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id query param required"})
		return
	}

	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session_id"})
		return
	}

	clips, err := h.store.ListClips(r.Context(), sessionID)
	if err != nil {
		slog.Error("clip: list", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if clips == nil {
		clips = []Clip{}
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *Handler) listRecentClips(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	clips, err := h.store.ListAllClips(r.Context(), limit)
	if err != nil {
		slog.Error("clip: list recent", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if clips == nil {
		clips = []Clip{}
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *Handler) serveClip(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if !strings.HasSuffix(filename, ".mp4") || strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, filepath.Join(h.clipsDir, filename))
}

func (h *Handler) deleteClip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid clip ID"})
		return
	}

	c, err := h.store.GetClip(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "clip not found"})
		return
	}

	if err := h.store.DeleteClip(r.Context(), id); err != nil {
		slog.Error("clip: delete", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	os.Remove(filepath.Join(h.clipsDir, c.Filename))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
