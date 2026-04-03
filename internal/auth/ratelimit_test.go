package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("1.2.3.4") {
		t.Fatal("request 4 should be denied")
	}

	// Different IP should be allowed
	if !rl.Allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)

	if !rl.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("second request should be denied")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("1.2.3.4") {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		want       string
	}{
		{"remote addr with port", "192.168.1.1:12345", "", "192.168.1.1"},
		{"remote addr without port", "192.168.1.1", "", "192.168.1.1"},
		{"xff single", "10.0.0.1:1234", "203.0.113.50", "203.0.113.50"},
		{"xff chain", "10.0.0.1:1234", "203.0.113.50, 70.41.3.18", "203.0.113.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     http.Header{},
			}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			got := ClientIP(r)
			if got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
