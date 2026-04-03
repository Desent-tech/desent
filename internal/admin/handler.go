package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"desent/internal/auth"
	"desent/internal/ingest"
)

// IngestManager is the subset of ingest.Manager used by admin endpoints.
type IngestManager interface {
	IsLive() bool
	EnabledQualities() []string
	SetQualities(names []string)
	FPSOverrides() map[string]int
	SetFPSOverrides(overrides map[string]int)
	GetPreset() string
	SetPreset(p string)
	ResolvedPreset() string
	Restart()
	StartedAt() time.Time
	GetStreamKey() string
	SetStreamKey(key string)
}

var allowedIconExts = []string{".png", ".jpg", ".jpeg", ".svg", ".ico"}

// UserKicker kicks a user from chat by user ID.
type UserKicker interface {
	Kick(userID int64)
}

type Handler struct {
	store     *Store
	ingestMgr IngestManager
	hlsDir    string
	defaultBW int
	dataDir   string
	kicker    UserKicker
}

func NewHandler(store *Store, ingestMgr IngestManager, hlsDir string, defaultBW int, dataDir string, kicker UserKicker) *Handler {
	return &Handler{
		store:     store,
		ingestMgr: ingestMgr,
		hlsDir:    hlsDir,
		defaultBW: defaultBW,
		dataDir:   dataDir,
		kicker:    kicker,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/admin/settings", mw(http.HandlerFunc(h.getSettings)))
	mux.Handle("PUT /api/admin/settings", mw(http.HandlerFunc(h.updateSettings)))
	mux.Handle("GET /api/admin/users", mw(http.HandlerFunc(h.listUsers)))
	mux.Handle("POST /api/admin/ban", mw(http.HandlerFunc(h.banUser)))
	mux.Handle("DELETE /api/admin/ban/{userId}", mw(http.HandlerFunc(h.unbanUser)))
	mux.Handle("GET /api/admin/stats", mw(http.HandlerFunc(h.getStats)))
	mux.Handle("GET /api/admin/qualities", mw(http.HandlerFunc(h.getQualities)))
	mux.Handle("PUT /api/admin/qualities", mw(http.HandlerFunc(h.updateQualities)))
	mux.Handle("PUT /api/admin/users/{userId}/role", mw(http.HandlerFunc(h.updateUserRole)))
	mux.Handle("POST /api/admin/icon", mw(http.HandlerFunc(h.uploadIcon)))
	mux.Handle("GET /api/admin/stream-key", mw(http.HandlerFunc(h.getStreamKey)))
	mux.Handle("POST /api/admin/stream-key/regenerate", mw(http.HandlerFunc(h.regenerateStreamKey)))
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		slog.Error("admin: get settings", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var kv map[string]string
	if err := json.NewDecoder(r.Body).Decode(&kv); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(kv) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty payload"})
		return
	}

	if err := h.store.UpdateSettings(r.Context(), kv); err != nil {
		slog.Error("admin: update settings", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		slog.Error("admin: get settings after update", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		slog.Error("admin: list users", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type banRequest struct {
	UserID int64  `json:"user_id"`
	Reason string `json:"reason"`
}

func (h *Handler) banUser(w http.ResponseWriter, r *http.Request) {
	var req banRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.UserID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	if claims.UserID == req.UserID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot ban yourself"})
		return
	}

	if err := h.store.BanUser(r.Context(), req.UserID, claims.UserID, req.Reason); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "user already banned"})
			return
		}
		slog.Error("admin: ban user", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	h.kicker.Kick(req.UserID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "banned"})
}

func (h *Handler) unbanUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("userId")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	if err := h.store.UnbanUser(r.Context(), userID); err != nil {
		if strings.Contains(err.Error(), "is not banned") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user is not banned"})
			return
		}
		slog.Error("admin: unban user", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbanned"})
}

// getStats returns server metrics, stream info, and viewer capacity.
func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	sys := GetSysInfo()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(h.ingestMgr.StartedAt()).Seconds()
	hlsDisk := dirSizeMB(h.hlsDir)
	qualities := h.ingestMgr.EnabledQualities()

	bw := h.getBandwidth(r)
	capacity := viewerCapacity(bw, qualities)

	writeJSON(w, http.StatusOK, map[string]any{
		"uptime_seconds":    int64(uptime),
		"cpu_usage_percent": math.Round(sys.CPUUsagePercent*10) / 10,
		"cpu_cores":         sys.CPUCores,
		"mem_total_mb":      math.Round(sys.MemTotalMB*10) / 10,
		"mem_used_mb":       math.Round(sys.MemUsedMB*10) / 10,
		"mem_used_percent":  math.Round(sys.MemUsedPercent*10) / 10,
		"mem_go_alloc_mb":   math.Round(float64(m.Alloc)/1024/1024*10) / 10,
		"num_goroutines":    runtime.NumGoroutine(),
		"hls_disk_usage_mb": math.Round(hlsDisk*10) / 10,
		"stream_live":       h.ingestMgr.IsLive(),
		"qualities":         qualities,
		"bandwidth_mbps":    bw,
		"viewer_capacity":   capacity,
	})
}

// getQualities returns enabled and available quality lists plus preset info.
func (h *Handler) getQualities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":           h.ingestMgr.EnabledQualities(),
		"available":         ingest.AllQualityNames(),
		"fps":               h.ingestMgr.FPSOverrides(),
		"preset":            h.ingestMgr.GetPreset(),
		"available_presets": ingest.ValidPresets,
		"auto_preset":       ingest.AutoPreset(),
		"cpu_cores":         runtime.NumCPU(),
	})
}

type qualitiesRequest struct {
	Enabled []string       `json:"enabled"`
	FPS     map[string]int `json:"fps,omitempty"`
	Preset  string         `json:"preset,omitempty"`
}

// updateQualities validates, persists, and applies quality changes.
func (h *Handler) updateQualities(w http.ResponseWriter, r *http.Request) {
	var req qualitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate names
	valid := ingest.FilterQualities(req.Enabled)
	if len(valid) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one valid quality required"})
		return
	}

	// Validate preset if provided
	if req.Preset != "" && !ingest.IsValidPreset(req.Preset) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid preset"})
		return
	}

	// Validate FPS overrides if provided
	for name, fps := range req.FPS {
		if fps != 30 && fps != 60 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fps must be 30 or 60"})
			return
		}
		q := ingest.QualityByName(name)
		if q == nil || !q.IsHD() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fps override only for HD qualities (720p+)"})
			return
		}
	}

	// Persist to DB
	names := make([]string, len(valid))
	for i, q := range valid {
		names[i] = q.Name
	}
	settingsToSave := map[string]string{
		"qualities_enabled": strings.Join(names, ","),
	}
	if len(req.FPS) > 0 {
		// Merge with current overrides so partial updates work
		merged := h.ingestMgr.FPSOverrides()
		for k, v := range req.FPS {
			merged[k] = v
		}
		settingsToSave["stream_fps"] = ingest.SerializeFPSOverrides(merged)
	}
	if req.Preset != "" {
		settingsToSave["ffmpeg_preset"] = req.Preset
	}
	err := h.store.UpdateSettings(r.Context(), settingsToSave)
	if err != nil {
		slog.Error("admin: persist qualities", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Apply
	h.ingestMgr.SetQualities(names)
	if len(req.FPS) > 0 {
		h.ingestMgr.SetFPSOverrides(req.FPS)
	}
	if req.Preset != "" {
		h.ingestMgr.SetPreset(req.Preset)
	}

	restarted := false
	if h.ingestMgr.IsLive() {
		h.ingestMgr.Restart()
		restarted = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":           names,
		"available":         ingest.AllQualityNames(),
		"fps":               h.ingestMgr.FPSOverrides(),
		"preset":            h.ingestMgr.GetPreset(),
		"available_presets": ingest.ValidPresets,
		"auto_preset":       ingest.AutoPreset(),
		"cpu_cores":         runtime.NumCPU(),
		"restarted":         restarted,
	})
}

// getBandwidth reads server_bandwidth_mbps from DB settings, falling back to env default.
func (h *Handler) getBandwidth(r *http.Request) int {
	settings, err := h.store.GetSettings(r.Context())
	if err == nil {
		if v, ok := settings["server_bandwidth_mbps"]; ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return n
			}
		}
	}
	return h.defaultBW
}

// viewerCapacity computes max viewers per quality given total bandwidth.
func viewerCapacity(bandwidthMbps int, qualityNames []string) map[string]int {
	cap := make(map[string]int, len(qualityNames))
	for _, name := range qualityNames {
		q := ingest.QualityByName(name)
		if q == nil {
			continue
		}
		bps := q.TotalBitrateMbps()
		if bps > 0 {
			cap[name] = int(float64(bandwidthMbps) / bps)
		}
	}
	return cap
}

// dirSizeMB returns total size of files in dir (recursive) in MB.
func dirSizeMB(dir string) float64 {
	var total int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return float64(total) / 1024.0 / 1024.0
}

type roleRequest struct {
	Role string `json:"role"`
}

func (h *Handler) updateUserRole(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user ID"})
		return
	}

	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := h.store.UpdateUserRole(r.Context(), userID, req.Role); err != nil {
		if strings.Contains(err.Error(), "invalid role") || strings.Contains(err.Error(), "not found or is admin") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		slog.Error("admin: update role", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "role updated"})
}

func (h *Handler) uploadIcon(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil { // 1MB max
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid form data"})
		return
	}

	file, header, err := r.FormFile("icon")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "icon file is required"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !isAllowedIconExt(ext) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported icon format, allowed: png, jpg, jpeg, svg, ico"})
		return
	}

	// Remove existing icon files
	for _, e := range allowedIconExts {
		os.Remove(filepath.Join(h.dataDir, "icon"+e))
	}

	dst, err := os.Create(filepath.Join(h.dataDir, "icon"+ext))
	if err != nil {
		slog.Error("admin: create icon file", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save icon"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		slog.Error("admin: write icon file", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save icon"})
		return
	}

	slog.Info("admin: icon updated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getStreamKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"stream_key": h.ingestMgr.GetStreamKey(),
		"rtmp_url":   "rtmp://{your-domain}:1935/live/" + h.ingestMgr.GetStreamKey(),
	})
}

func (h *Handler) regenerateStreamKey(w http.ResponseWriter, r *http.Request) {
	key, err := generateStreamKey()
	if err != nil {
		slog.Error("admin: generate stream key", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate key"})
		return
	}

	if err := h.store.UpdateSettings(r.Context(), map[string]string{"stream_key": key}); err != nil {
		slog.Error("admin: persist stream key", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	h.ingestMgr.SetStreamKey(key)

	if h.ingestMgr.IsLive() {
		h.ingestMgr.Restart()
	}

	slog.Info("admin: stream key regenerated")
	writeJSON(w, http.StatusOK, map[string]string{
		"stream_key": key,
		"rtmp_url":   "rtmp://{your-domain}:1935/live/" + key,
	})
}

// GenerateStreamKey creates a random 32-char hex key.
func generateStreamKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isAllowedIconExt(ext string) bool {
	for _, a := range allowedIconExts {
		if a == ext {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
