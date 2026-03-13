package ingest

import (
	"fmt"
	"strings"
)

func buildFFmpegArgs(rtmpURL, hlsDir string, qualities []Quality, preset string) []string {
	args := []string{
		"-listen", "1",
		"-i", rtmpURL,
	}

	// Map streams: one video + one audio per quality
	for range qualities {
		args = append(args, "-map", "0:v:0", "-map", "0:a:0")
	}

	// Shared video codec settings
	args = append(args, "-c:v", "libx264", "-preset", preset, "-sc_threshold", "0")

	// Per-variant video settings
	for i, q := range qualities {
		gop := q.FPS * 2
		args = append(args,
			fmt.Sprintf("-filter:v:%d", i), fmt.Sprintf("scale=%d:%d,fps=%d", q.Width, q.Height, q.FPS),
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", q.VideoBitrateK),
			fmt.Sprintf("-maxrate:v:%d", i), fmt.Sprintf("%dk", q.MaxRateK),
			fmt.Sprintf("-bufsize:v:%d", i), fmt.Sprintf("%dk", q.BufSizeK),
			fmt.Sprintf("-g:v:%d", i), fmt.Sprintf("%d", gop),
			fmt.Sprintf("-keyint_min:v:%d", i), fmt.Sprintf("%d", gop),
		)
	}

	// Audio
	args = append(args, "-c:a", "aac", "-ar", "48000")
	for i, q := range qualities {
		args = append(args, fmt.Sprintf("-b:a:%d", i), fmt.Sprintf("%dk", q.AudioBitrateK))
	}

	// var_stream_map
	var parts []string
	for i, q := range qualities {
		parts = append(parts, fmt.Sprintf("v:%d,a:%d,name:%s", i, i, q.Name))
	}
	args = append(args, "-var_stream_map", strings.Join(parts, " "))

	// HLS output
	args = append(args,
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+independent_segments+temp_file+program_date_time",
		"-master_pl_name", "master.m3u8",
		"-hls_segment_filename", fmt.Sprintf("%s/%%v/seg%%03d.ts", hlsDir),
		fmt.Sprintf("%s/%%v/index.m3u8", hlsDir),
	)

	return args
}
