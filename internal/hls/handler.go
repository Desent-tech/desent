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
	hlsDir string
	cache  *SegmentCache
}

func NewHandler(hlsDir string, cache *SegmentCache) *Handler {
	return &Handler{
		hlsDir: hlsDir,
		cache:  cache,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /live/", h.serveHLS())
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

func (h *Handler) StreamStatusHandler(lc LiveChecker, ql QualityLister, fp FPSProvider, tp TitleProvider, vc ViewerCounter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"live":      lc.IsLive(),
			"qualities": ql.EnabledQualities(),
			"fps":       fp.QualityFPS(),
			"title":     tp.GetStreamTitle(r.Context()),
			"viewers":   vc.ViewerCount(),
		})
	}
}
