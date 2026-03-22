package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"desent/internal/auth"
)

var allowedIconExts = []string{".png", ".jpg", ".jpeg", ".svg", ".ico"}

type Handler struct {
	store        *Store
	authStore    *auth.Store
	tokenService *auth.TokenService
	bcryptCost   int
	dataDir      string
}

func NewHandler(store *Store, authStore *auth.Store, tokenService *auth.TokenService, bcryptCost int, dataDir string) *Handler {
	return &Handler{
		store:        store,
		authStore:    authStore,
		tokenService: tokenService,
		bcryptCost:   bcryptCost,
		dataDir:      dataDir,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup/status", h.status)
	mux.HandleFunc("POST /api/setup/complete", h.complete)
	mux.HandleFunc("GET /api/icon", h.serveIcon)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	required, err := h.store.SetupRequired(r.Context())
	if err != nil {
		slog.Error("setup: check status", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_required": required})
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	required, err := h.store.SetupRequired(r.Context())
	if err != nil {
		slog.Error("setup: check status", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if !required {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup already completed"})
		return
	}

	if err := r.ParseMultipartForm(1 << 20); err != nil { // 1MB max
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid form data"})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}
	if len(username) < 3 || len(username) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 3-32 characters"})
		return
	}
	if len(password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 8 characters"})
		return
	}

	// Handle icon upload
	file, header, err := r.FormFile("icon")
	if err == nil {
		defer file.Close()
		if err := saveIcon(h.dataDir, file, header.Filename); err != nil {
			slog.Error("setup: save icon", "err", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	// Create admin user
	user, err := h.authStore.CreateUser(r.Context(), username, password, h.bcryptCost)
	if err != nil {
		slog.Error("setup: create user", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	token, err := h.tokenService.Generate(user)
	if err != nil {
		slog.Error("setup: generate token", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	slog.Info("setup completed", "username", username)
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"role":  user.Role,
	})
}

func (h *Handler) serveIcon(w http.ResponseWriter, r *http.Request) {
	path, err := findIcon(h.dataDir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

// saveIcon writes the uploaded icon to dataDir/icon{ext}, removing any prior icon files.
func saveIcon(dataDir string, file io.Reader, filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if !isAllowedIconExt(ext) {
		return fmt.Errorf("unsupported icon format, allowed: png, jpg, jpeg, svg, ico")
	}

	// Remove existing icon files
	removeIcons(dataDir)

	dst, err := os.Create(filepath.Join(dataDir, "icon"+ext))
	if err != nil {
		return fmt.Errorf("create icon file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("write icon file: %w", err)
	}
	return nil
}

// findIcon looks for an icon file in dataDir with any allowed extension.
func findIcon(dataDir string) (string, error) {
	for _, ext := range allowedIconExts {
		path := filepath.Join(dataDir, "icon"+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no icon found")
}

func removeIcons(dataDir string) {
	for _, ext := range allowedIconExts {
		os.Remove(filepath.Join(dataDir, "icon"+ext))
	}
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
