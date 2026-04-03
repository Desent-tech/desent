package auth

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter tracks request counts per IP with a sliding window.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
	limit   int
	window  time.Duration
}

type rlEntry struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter creates a rate limiter allowing limit requests per window per IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rlEntry),
		limit:   limit,
		window:  window,
	}
}

// Allow returns true if the request from ip is within the rate limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.After(e.resetAt) {
		rl.entries[ip] = &rlEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	e.count++
	return e.count <= rl.limit
}

// StartCleanup periodically removes expired entries.
func (rl *RateLimiter) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, e := range rl.entries {
				if now.After(e.resetAt) {
					delete(rl.entries, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// ClientIP extracts the client IP from the request,
// checking X-Forwarded-For for reverse proxy setups.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the original client
		if i := net.ParseIP(xff); i != nil {
			return i.String()
		}
		// X-Forwarded-For can be comma-separated
		for _, part := range splitComma(xff) {
			if ip := net.ParseIP(part); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func splitComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	part := trimSpace(s[start:])
	if part != "" {
		result = append(result, part)
	}
	return result
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}
