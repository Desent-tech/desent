package ingest

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// ValidPresets lists all x264 presets exposed to admins (from fastest to slowest).
var ValidPresets = []string{"auto", "ultrafast", "superfast", "veryfast", "faster", "fast", "medium"}

type Config struct {
	FFmpegPath  string
	RTMPAddr    string
	StreamKey   string
	HLSDir      string
	VODDir      string
	OnStreamEnd func() // called after FFmpeg exits (stream ends)
}

type Manager struct {
	cfg            Config
	mu             sync.Mutex
	cmd            *exec.Cmd
	running        bool
	qualities      []Quality
	fpsOverrides   map[string]int
	preset         string
	startedAt      time.Time
	currentVODPath string
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:          cfg,
		qualities:    FilterQualities(DefaultQualities),
		fpsOverrides: DefaultFPSOverrides(),
		preset:       "auto",
	}
}

// AutoPreset returns the best x264 preset for the current CPU.
func AutoPreset() string {
	cores := runtime.NumCPU()
	switch {
	case cores >= 8:
		return "medium"
	case cores >= 4:
		return "fast"
	case cores >= 2:
		return "veryfast"
	default:
		return "ultrafast"
	}
}

// IsValidPreset checks if a preset string is in ValidPresets.
func IsValidPreset(p string) bool {
	for _, v := range ValidPresets {
		if v == p {
			return true
		}
	}
	return false
}

// SetStreamKey updates the stream key used for RTMP authentication.
func (m *Manager) SetStreamKey(key string) {
	m.mu.Lock()
	m.cfg.StreamKey = key
	m.mu.Unlock()
}

// GetStreamKey returns the current stream key.
func (m *Manager) GetStreamKey() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg.StreamKey
}

// CurrentVODPath returns the path to the current VOD file being recorded.
func (m *Manager) CurrentVODPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentVODPath
}

// SetPreset updates the x264 preset. Use "auto" for CPU-based auto-detection.
func (m *Manager) SetPreset(p string) {
	m.mu.Lock()
	m.preset = p
	m.mu.Unlock()
}

// GetPreset returns the current preset setting (may be "auto").
func (m *Manager) GetPreset() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preset
}

// ResolvedPreset returns the effective preset (resolves "auto" to a concrete value).
func (m *Manager) ResolvedPreset() string {
	m.mu.Lock()
	p := m.preset
	m.mu.Unlock()
	if p == "auto" {
		return AutoPreset()
	}
	return p
}

// SetFPSOverrides merges per-quality FPS overrides. Only HD qualities (720p+)
// with values 30 or 60 are accepted.
func (m *Manager) SetFPSOverrides(overrides map[string]int) {
	m.mu.Lock()
	for _, q := range AllQualities {
		if !q.IsHD() {
			continue
		}
		if fps, ok := overrides[q.Name]; ok && (fps == 30 || fps == 60) {
			m.fpsOverrides[q.Name] = fps
		}
	}
	m.mu.Unlock()
}

// FPSOverrides returns a copy of the current per-quality FPS map.
func (m *Manager) FPSOverrides() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string]int, len(m.fpsOverrides))
	for k, v := range m.fpsOverrides {
		cp[k] = v
	}
	return cp
}

// QualityFPS returns the effective FPS for each enabled quality.
// HD qualities use overrides, SD qualities use their preset default.
func (m *Manager) QualityFPS() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]int, len(m.qualities))
	for _, q := range m.qualities {
		if fps, ok := m.fpsOverrides[q.Name]; ok {
			result[q.Name] = fps
		} else {
			result[q.Name] = q.FPS
		}
	}
	return result
}

// SetQualities updates the enabled quality list. Never allows an empty set.
func (m *Manager) SetQualities(names []string) {
	q := FilterQualities(names)
	if len(q) == 0 {
		return
	}
	m.mu.Lock()
	m.qualities = q
	m.mu.Unlock()
}

// EnabledQualities returns the names of currently enabled qualities.
func (m *Manager) EnabledQualities() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, len(m.qualities))
	for i, q := range m.qualities {
		names[i] = q.Name
	}
	return names
}

// Restart sends SIGTERM to the current FFmpeg process.
// The Run() loop will automatically restart it with the latest qualities.
func (m *Manager) Restart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)
	}
}

// StartedAt returns when the manager was started.
func (m *Manager) StartedAt() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startedAt
}

// Run starts the FFmpeg ingest loop. Blocks until ctx is cancelled.
// Automatically restarts FFmpeg when the stream disconnects.
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	m.startedAt = time.Now()
	m.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			m.stop()
			return
		default:
		}

		slog.Info("ingest: waiting for RTMP stream", "addr", m.cfg.RTMPAddr, "key", m.cfg.StreamKey)
		if err := m.runFFmpeg(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("ingest: FFmpeg exited", "err", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (m *Manager) runFFmpeg(ctx context.Context) error {
	rtmpURL := "rtmp://" + m.cfg.RTMPAddr + "/live/" + m.cfg.StreamKey

	// Snapshot qualities, FPS overrides, and preset under lock
	m.mu.Lock()
	quals := make([]Quality, len(m.qualities))
	copy(quals, m.qualities)
	overrides := make(map[string]int, len(m.fpsOverrides))
	for k, v := range m.fpsOverrides {
		overrides[k] = v
	}
	preset := m.preset
	m.mu.Unlock()

	if preset == "auto" {
		preset = AutoPreset()
	}

	quals = ApplyFPSOverrides(quals, overrides)

	// Generate VOD path if VOD directory is configured
	var vodPath string
	if m.cfg.VODDir != "" {
		vodPath = filepath.Join(m.cfg.VODDir, fmt.Sprintf("%d.mp4", time.Now().Unix()))
	}

	cmd := exec.CommandContext(ctx, m.cfg.FFmpegPath, buildFFmpegArgs(rtmpURL, m.cfg.HLSDir, quals, preset, vodPath)...)
	cmd.WaitDelay = 5 * time.Second

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.cmd = cmd
	m.running = true
	m.currentVODPath = vodPath
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		m.mu.Lock()
		m.running = false
		m.currentVODPath = ""
		m.mu.Unlock()
		return err
	}

	slog.Info("ingest: FFmpeg started", "pid", cmd.Process.Pid, "qualities", qualityNames(quals), "preset", preset, "vod", vodPath)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Debug("ffmpeg", "line", scanner.Text())
		}
	}()

	err = cmd.Wait()

	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	if m.cfg.OnStreamEnd != nil {
		m.cfg.OnStreamEnd()
	}

	return err
}

func (m *Manager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Signal(syscall.SIGTERM)
	}
}

// IsLive reports whether FFmpeg is currently running (stream is active).
func (m *Manager) IsLive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func qualityNames(qs []Quality) []string {
	names := make([]string, len(qs))
	for i, q := range qs {
		names[i] = q.Name
	}
	return names
}
