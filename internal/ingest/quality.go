package ingest

import (
	"strconv"
	"strings"
)

// Quality defines a transcoding preset.
type Quality struct {
	Name          string
	Width         int
	Height        int
	FPS           int
	VideoBitrateK int
	MaxRateK      int
	BufSizeK      int
	AudioBitrateK int
}

// HDMinHeight is the minimum height for qualities that support FPS override.
const HDMinHeight = 720

// TotalBitrateMbps returns the combined video+audio bitrate in Mbps.
func (q Quality) TotalBitrateMbps() float64 {
	return float64(q.VideoBitrateK+q.AudioBitrateK) / 1000.0
}

// IsHD returns true if the quality supports per-quality FPS override (720p+).
func (q Quality) IsHD() bool {
	return q.Height >= HDMinHeight
}

// AllQualities lists every available preset, highest to lowest.
var AllQualities = []Quality{
	{Name: "2160p", Width: 3840, Height: 2160, FPS: 60, VideoBitrateK: 20000, MaxRateK: 21400, BufSizeK: 40000, AudioBitrateK: 192},
	{Name: "1440p", Width: 2560, Height: 1440, FPS: 60, VideoBitrateK: 12000, MaxRateK: 12840, BufSizeK: 24000, AudioBitrateK: 160},
	{Name: "1080p", Width: 1920, Height: 1080, FPS: 60, VideoBitrateK: 6000, MaxRateK: 6420, BufSizeK: 12000, AudioBitrateK: 128},
	{Name: "720p", Width: 1280, Height: 720, FPS: 60, VideoBitrateK: 4000, MaxRateK: 4280, BufSizeK: 8000, AudioBitrateK: 128},
	{Name: "480p", Width: 854, Height: 480, FPS: 30, VideoBitrateK: 1500, MaxRateK: 1605, BufSizeK: 3000, AudioBitrateK: 96},
	{Name: "360p", Width: 640, Height: 360, FPS: 30, VideoBitrateK: 600, MaxRateK: 642, BufSizeK: 1200, AudioBitrateK: 64},
}

// DefaultQualities are enabled out of the box.
var DefaultQualities = []string{"1080p", "720p", "480p", "360p"}

// QualityByName returns a pointer to the preset or nil.
func QualityByName(name string) *Quality {
	for i := range AllQualities {
		if AllQualities[i].Name == name {
			return &AllQualities[i]
		}
	}
	return nil
}

// AllQualityNames returns the names of all available presets.
func AllQualityNames() []string {
	names := make([]string, len(AllQualities))
	for i, q := range AllQualities {
		names[i] = q.Name
	}
	return names
}

// DefaultFPSOverrides returns the default FPS for each HD quality.
func DefaultFPSOverrides() map[string]int {
	m := make(map[string]int)
	for _, q := range AllQualities {
		if q.IsHD() {
			m[q.Name] = q.FPS
		}
	}
	return m
}

// ApplyFPSOverrides returns a copy of qualities with FPS overridden per the map.
// Only HD qualities (720p+) are affected. SD qualities keep their preset FPS.
func ApplyFPSOverrides(qualities []Quality, overrides map[string]int) []Quality {
	out := make([]Quality, len(qualities))
	copy(out, qualities)
	for i := range out {
		if fps, ok := overrides[out[i].Name]; ok && (fps == 30 || fps == 60) {
			out[i].FPS = fps
		}
	}
	return out
}

// SerializeFPSOverrides encodes the map as "name:fps,name:fps,...".
func SerializeFPSOverrides(overrides map[string]int) string {
	var parts []string
	for _, q := range AllQualities {
		if fps, ok := overrides[q.Name]; ok {
			parts = append(parts, q.Name+":"+strconv.Itoa(fps))
		}
	}
	return strings.Join(parts, ",")
}

// ParseFPSOverrides decodes "name:fps,name:fps,..." into a map.
func ParseFPSOverrides(s string) map[string]int {
	m := make(map[string]int)
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		fps, err := strconv.Atoi(kv[1])
		if err != nil || (fps != 30 && fps != 60) {
			continue
		}
		if QualityByName(kv[0]) != nil {
			m[kv[0]] = fps
		}
	}
	return m
}

// FilterQualities returns the Quality entries matching the given names,
// preserving the order from AllQualities. Unknown names are silently skipped.
func FilterQualities(enabled []string) []Quality {
	set := make(map[string]bool, len(enabled))
	for _, n := range enabled {
		set[n] = true
	}
	var out []Quality
	for _, q := range AllQualities {
		if set[q.Name] {
			out = append(out, q)
		}
	}
	return out
}
