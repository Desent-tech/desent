package hls

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Handler struct {
	hlsDir  string
	cache   *SegmentCache
	vodDir  string
	dataDir string
}

func NewHandler(hlsDir string, cache *SegmentCache, vodDir, dataDir string) *Handler {
	return &Handler{
		hlsDir:  hlsDir,
		cache:   cache,
		vodDir:  vodDir,
		dataDir: dataDir,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /live/", h.serveHLS())
	mux.HandleFunc("GET /vods/{filename}", h.serveVOD)
	mux.HandleFunc("GET /api/stream/thumbnail", h.serveThumbnail)
}

func (h *Handler) serveHLS() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// MIME types and cache
		if strings.HasSuffix(path, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if strings.HasSuffix(path, ".ts") {
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		relPath := strings.TrimPrefix(path, "/live/")

		// Try cache first.
		if data, modTime, ok := h.cache.Get(relPath); ok {
			http.ServeContent(w, r, relPath, modTime, bytes.NewReader(data))
			return
		}

		// Fallback to disk.
		absPath := filepath.Join(h.hlsDir, filepath.FromSlash(relPath))
		info, err := os.Stat(absPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(absPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, relPath, info.ModTime(), f)
	})
}

func (h *Handler) serveVOD(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if !strings.HasSuffix(filename, ".mp4") || strings.Contains(filename, "/") || strings.Contains(filename, "..") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	absPath := filepath.Join(h.vodDir, filename)
	http.ServeFile(w, r, absPath)
}

func (h *Handler) serveThumbnail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache")
	absPath := filepath.Join(h.dataDir, "thumb.jpg")
	if _, err := os.Stat(absPath); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, absPath)
}

// StreamStatusHandler returns a handler reporting whether the stream is live.
type LiveChecker interface {
	IsLive() bool
}

// QualityLister reports currently enabled quality names.
type QualityLister interface {
	EnabledQualities() []string
}

// FPSProvider reports the effective FPS for each enabled quality.
type FPSProvider interface {
	QualityFPS() map[string]int
}

// TitleProvider returns the current stream title.
type TitleProvider interface {
	GetStreamTitle(ctx context.Context) string
}

// ViewerCounter reports the current number of connected viewers.
type ViewerCounter interface {
	ViewerCount() int
}

// CategoryProvider returns stream category and tags.
type CategoryProvider interface {
	GetStreamCategory(ctx context.Context) string
	GetStreamTags(ctx context.Context) string
}

func (h *Handler) StreamStatusHandler(lc LiveChecker, ql QualityLister, fp FPSProvider, tp TitleProvider, vc ViewerCounter, cp CategoryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"live":      lc.IsLive(),
			"qualities": ql.EnabledQualities(),
			"fps":       fp.QualityFPS(),
			"title":     tp.GetStreamTitle(r.Context()),
			"viewers":   vc.ViewerCount(),
			"category":  cp.GetStreamCategory(r.Context()),
			"tags":      cp.GetStreamTags(r.Context()),
		})
	}
}
