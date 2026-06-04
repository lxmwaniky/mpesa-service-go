package http

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lxmwaniky/mpesa-service-go/config"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter    *rate.Limiter
	lastActive time.Time
}

type ipLimiter struct {
	mu  sync.Mutex
	ips map[string]*limiterEntry
}

var limiterStore = &ipLimiter{
	ips: make(map[string]*limiterEntry),
}

func init() {
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			limiterStore.cleanup()
		}
	}()
}

func (l *ipLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.ips[ip]
	if !exists {
		entry = &limiterEntry{
			limiter:    rate.NewLimiter(rate.Every(10*time.Second), 3),
			lastActive: time.Now(),
		}
		l.ips[ip] = entry
	} else {
		entry.lastActive = time.Now()
	}

	return entry.limiter
}

func (l *ipLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, entry := range l.ips {
		if now.Sub(entry.lastActive) > 10*time.Minute {
			delete(l.ips, ip)
		}
	}
}

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ip string
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if parts := strings.Split(xff, ","); len(parts) > 0 {
				ip = strings.TrimSpace(parts[0])
			}
		}
		if ip == "" {
			ip = r.Header.Get("X-Real-IP")
		}
		if ip == "" {
			var err error
			ip, _, err = net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
		}

		limiter := limiterStore.getLimiter(ip)
		if !limiter.Allow() {
			slog.Warn("rate limit exceeded", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "Too many requests. Please slow down."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func APIKeyAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" || key != cfg.AppAPIKey {
				slog.Warn("unauthorized api access attempt",
					"ip", r.RemoteAddr,
					"path", r.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		if rw.status >= 500 {
			slog.Error("request failed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		} else if rw.status >= 400 {
			slog.Warn("request client error",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		} else {
			slog.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", duration.Milliseconds(),
				"ip", r.RemoteAddr,
			)
		}
	})
}

func EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}