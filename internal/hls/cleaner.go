package hls

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StartCleaner runs a background goroutine that removes stale .ts segments
// left behind after FFmpeg crashes or restarts.
func StartCleaner(ctx context.Context, hlsDir string, interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanSegments(hlsDir, maxAge)
			}
		}
	}()
}

// CleanAll removes all .ts and .m3u8 files from the HLS directory.
// Used on stream end and server startup to prevent stale segments from being served.
func CleanAll(hlsDir string) {
	subdirs, err := os.ReadDir(hlsDir)
	if err != nil {
		return
	}
	removed := 0
	for _, sub := range subdirs {
		if !sub.IsDir() {
			// Top-level files (e.g. master.m3u8)
			if strings.HasSuffix(sub.Name(), ".m3u8") || strings.HasSuffix(sub.Name(), ".ts") {
				path := filepath.Join(hlsDir, sub.Name())
				if os.Remove(path) == nil {
					removed++
				}
			}
			continue
		}
		dir := filepath.Join(hlsDir, sub.Name())
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".m3u8") {
				path := filepath.Join(dir, name)
				if os.Remove(path) == nil {
					removed++
				}
			}
		}
	}
	if removed > 0 {
		slog.Info("cleaner: removed all HLS files", "count", removed)
	}
}

func cleanSegments(hlsDir string, maxAge time.Duration) {
	// Scan subdirectories dynamically instead of hardcoding quality names
	subdirs, err := os.ReadDir(hlsDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, sub := range subdirs {
		if !sub.IsDir() {
			continue
		}
		dir := filepath.Join(hlsDir, sub.Name())
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ts") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				path := filepath.Join(dir, entry.Name())
				if err := os.Remove(path); err == nil {
					slog.Debug("cleaner: removed stale segment", "path", path)
				}
			}
		}
	}
}
