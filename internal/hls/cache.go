package hls

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type cacheEntry struct {
	data    []byte
	modTime time.Time
}

// SegmentCache watches an HLS directory for .ts and .m3u8 files, keeping them
// in memory so viewer requests never hit disk.
type SegmentCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry // key: relative path like "720p/seg001.ts"
	size    int64                 // current total bytes cached
	maxSize int64                 // cap from HLS_CACHE_MB

	watcher *fsnotify.Watcher
	hlsDir  string
}

// NewSegmentCache creates a cache backed by fsnotify. maxSizeMB is the memory
// cap; 0 means unlimited (not recommended).
func NewSegmentCache(hlsDir string, maxSizeMB int) (*SegmentCache, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &SegmentCache{
		entries: make(map[string]cacheEntry),
		maxSize: int64(maxSizeMB) * 1024 * 1024,
		watcher: w,
		hlsDir:  hlsDir,
	}, nil
}

// Start begins watching hlsDir for file events. It blocks until ctx is
// cancelled, so call it in a goroutine.
func (c *SegmentCache) Start(ctx context.Context) error {
	// Watch top-level dir so we catch new quality subdirs.
	if err := c.watcher.Add(c.hlsDir); err != nil {
		return err
	}

	// Watch any existing subdirs (quality folders created at startup).
	entries, _ := os.ReadDir(c.hlsDir)
	for _, e := range entries {
		if e.IsDir() {
			_ = c.watcher.Add(filepath.Join(c.hlsDir, e.Name()))
		}
	}

	// Pre-load existing files (startup with segments already on disk).
	c.preload()

	slog.Info("segment cache started", "dir", c.hlsDir, "maxMB", c.maxSize/(1024*1024))

	for {
		select {
		case <-ctx.Done():
			c.watcher.Close()
			return nil

		case ev, ok := <-c.watcher.Events:
			if !ok {
				return nil
			}
			c.handleEvent(ev)

		case err, ok := <-c.watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("fsnotify error", "err", err)
		}
	}
}

// Get returns cached file data and its modification time.
// Returns (nil, time.Time{}, false) on cache miss.
func (c *SegmentCache) Get(relPath string) ([]byte, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[relPath]
	if !ok {
		return nil, time.Time{}, false
	}
	return e.data, e.modTime, true
}

func (c *SegmentCache) handleEvent(ev fsnotify.Event) {
	// If a new directory is created (new quality folder), watch it.
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			_ = c.watcher.Add(ev.Name)
			slog.Debug("cache: watching new subdir", "path", ev.Name)
			return
		}
	}

	rel := c.relPath(ev.Name)
	if rel == "" {
		return
	}
	if !isHLSFile(rel) {
		return
	}

	switch {
	case ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write):
		c.loadFile(ev.Name, rel)
	case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
		c.evict(rel)
	}
}

func (c *SegmentCache) loadFile(absPath, rel string) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return // file may have been removed already
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update size accounting: subtract old entry size if overwriting.
	if old, exists := c.entries[rel]; exists {
		c.size -= int64(len(old.data))
	}

	c.entries[rel] = cacheEntry{data: data, modTime: info.ModTime()}
	c.size += int64(len(data))

	// Evict oldest .ts entries if over budget.
	c.evictIfNeeded()

	slog.Debug("cache: loaded", "path", rel, "bytes", len(data), "totalMB", c.size/(1024*1024))
}

func (c *SegmentCache) evict(rel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[rel]; ok {
		c.size -= int64(len(e.data))
		delete(c.entries, rel)
		slog.Debug("cache: evicted", "path", rel)
	}
}

// evictIfNeeded removes the oldest .ts entries until size <= maxSize.
// Must be called with c.mu held.
func (c *SegmentCache) evictIfNeeded() {
	if c.maxSize <= 0 || c.size <= c.maxSize {
		return
	}

	// Collect .ts entries sorted by modTime ascending.
	type kv struct {
		key     string
		modTime time.Time
		size    int
	}
	var candidates []kv
	for k, e := range c.entries {
		if strings.HasSuffix(k, ".ts") {
			candidates = append(candidates, kv{k, e.modTime, len(e.data)})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.Before(candidates[j].modTime)
	})

	for _, cand := range candidates {
		if c.size <= c.maxSize {
			break
		}
		c.size -= int64(cand.size)
		delete(c.entries, cand.key)
		slog.Debug("cache: evicted for space", "path", cand.key)
	}
}

// Purge removes all entries from the cache.
func (c *SegmentCache) Purge() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]cacheEntry)
	c.size = 0
	slog.Debug("cache: purged all entries")
}

func (c *SegmentCache) preload() {
	filepath.WalkDir(c.hlsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel := c.relPath(path)
		if rel == "" || !isHLSFile(rel) {
			return nil
		}
		c.loadFile(path, rel)
		return nil
	})
}

func (c *SegmentCache) relPath(absPath string) string {
	rel, err := filepath.Rel(c.hlsDir, absPath)
	if err != nil {
		return ""
	}
	// Normalise to forward slashes for map keys.
	return filepath.ToSlash(rel)
}

func isHLSFile(name string) bool {
	return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".m3u8")
}
