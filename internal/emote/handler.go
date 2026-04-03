package emote

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var validCode = regexp.MustCompile(`^[a-zA-Z0-9_]{2,32}$`)

var allowedExts = map[string]bool{
	".png":  true,
	".gif":  true,
	".webp": true,
}

type Handler struct {
	store     *Store
	emotesDir string
}

func NewHandler(store *Store, emotesDir string) *Handler {
	return &Handler{
		store:     store,
		emotesDir: emotesDir,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/emotes", h.listEmotes)
	mux.HandleFunc("GET /api/emotes/{filename}", h.serveEmote)
}

func (h *Handler) RegisterAdminRoutes(mux *http.ServeMux, mw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/admin/emotes", mw(http.HandlerFunc(h.uploadEmote)))
	mux.Handle("DELETE /api/admin/emotes/{id}", mw(http.HandlerFunc(h.deleteEmote)))
}

func (h *Handler) listEmotes(w http.ResponseWriter, r *http.Request) {
	emotes, err := h.store.ListEmotes(r.Context())
	if err != nil {
		slog.Error("emote: list", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if emotes == nil {
		emotes = []Emote{}
	}
	writeJSON(w, http.StatusOK, emotes)
}

func (h *Handler) serveEmote(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(h.emotesDir, filename))
}

func (h *Handler) uploadEmote(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(256 << 10); err != nil { // 256KB max
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large (max 256KB)"})
		return
	}

	code := r.FormValue("code")
	if !validCode.MatchString(code) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code must be 2-32 alphanumeric characters"})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image file is required"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExts[ext] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "allowed formats: png, gif, webp"})
		return
	}

	filename := code + ext
	dstPath := filepath.Join(h.emotesDir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		slog.Error("emote: create file", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save emote"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(dstPath)
		slog.Error("emote: write file", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save emote"})
		return
	}

	id, err := h.store.CreateEmote(r.Context(), code, filename)
	if err != nil {
		os.Remove(dstPath)
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "emote code already exists"})
			return
		}
		slog.Error("emote: create", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	slog.Info("emote: uploaded", "code", code, "filename", filename)
	writeJSON(w, http.StatusCreated, Emote{
		ID:       id,
		Code:     code,
		Filename: filename,
	})
}

func (h *Handler) deleteEmote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid emote ID"})
		return
	}

	emote, err := h.store.GetEmote(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "emote not found"})
		return
	}

	if err := h.store.DeleteEmote(r.Context(), id); err != nil {
		slog.Error("emote: delete", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	os.Remove(filepath.Join(h.emotesDir, emote.Filename))
	slog.Info("emote: deleted", "code", emote.Code)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
