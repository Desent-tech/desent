package thumbnail

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LiveChecker reports whether the stream is currently live.
type LiveChecker interface {
	IsLive() bool
}

type Generator struct {
	hlsDir     string
	dataDir    string
	ffmpegPath string
	liveCheck  LiveChecker
	mu         sync.Mutex
}

func NewGenerator(hlsDir, dataDir, ffmpegPath string, lc LiveChecker) *Generator {
	return &Generator{
		hlsDir:     hlsDir,
		dataDir:    dataDir,
		ffmpegPath: ffmpegPath,
		liveCheck:  lc,
	}
}

// Run periodically captures a thumbnail from the live stream.
// Blocks until ctx is cancelled.
func (g *Generator) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if g.liveCheck.IsLive() {
				if err := g.capture(); err != nil {
					slog.Debug("thumbnail: capture failed", "err", err)
				}
			}
		}
	}
}

// CopyForSession copies the current thumbnail as a VOD thumbnail.
func (g *Generator) CopyForSession(sessionID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src := filepath.Join(g.dataDir, "thumb.jpg")
	if _, err := os.Stat(src); err != nil {
		return
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return
	}

	dst := filepath.Join(g.dataDir, "vods", fmt.Sprintf("%d_thumb.jpg", sessionID))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		slog.Error("thumbnail: copy for session", "err", err)
	}
}

func (g *Generator) capture() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Find the latest .ts segment file across all quality dirs
	segment := g.findLatestSegment()
	if segment == "" {
		return fmt.Errorf("no segments found")
	}

	outPath := filepath.Join(g.dataDir, "thumb.jpg")
	tmpPath := outPath + ".tmp"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, g.ffmpegPath,
		"-y",
		"-i", segment,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale=640:-1",
		tmpPath,
	)
	cmd.Stderr = nil
	cmd.Stdout = nil

	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

func (g *Generator) findLatestSegment() string {
	var segments []struct {
		path    string
		modTime time.Time
	}

	filepath.Walk(g.hlsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".ts") {
			segments = append(segments, struct {
				path    string
				modTime time.Time
			}{path, info.ModTime()})
		}
		return nil
	})

	if len(segments) == 0 {
		return ""
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].modTime.After(segments[j].modTime)
	})

	return segments[0].path
}
