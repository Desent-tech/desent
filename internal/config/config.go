package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr            string
	DBPath              string
	JWTSecret           string
	BcryptCost          int
	StreamKey           string
	HLSDir              string
	RTMPAddr            string
	FFmpegPath          string
	ChatMaxMsgLen       int
	ChatRateLimitMS     int
	ServerBandwidthMbps int
	HLSCacheMB          int
	LogLevel            string
}

func Load() *Config {
	return &Config{
		HTTPAddr:            getEnv("HTTP_ADDR", ":8080"),
		DBPath:              getEnv("DB_PATH", "./desent.db"),
		JWTSecret:           getEnv("JWT_SECRET", ""),
		BcryptCost:          getEnvInt("BCRYPT_COST", 12),
		StreamKey:           getEnv("STREAM_KEY", "live"),
		HLSDir:              getEnv("HLS_DIR", "/tmp/hls"),
		RTMPAddr:            getEnv("RTMP_ADDR", "0.0.0.0:1935"),
		FFmpegPath:          getEnv("FFMPEG_PATH", "ffmpeg"),
		ChatMaxMsgLen:       getEnvInt("CHAT_MAX_MSG_LEN", 500),
		ChatRateLimitMS:     getEnvInt("CHAT_RATE_LIMIT_MS", 500),
		ServerBandwidthMbps: getEnvInt("SERVER_BANDWIDTH_MBPS", 100),
		HLSCacheMB:          getEnvInt("HLS_CACHE_MB", 128),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
	}
}

// ParseLogLevel converts a log level string to slog.Level.
// Supported values: debug, info, warn, error. Defaults to info.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
