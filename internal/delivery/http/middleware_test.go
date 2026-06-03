package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEnableCORS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router := EnableCORS(handler)

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	headers := []struct {
		key string
		val string
	}{
		{"Access-Control-Allow-Origin", "*"},
		{"Access-Control-Allow-Methods", "POST, GET, OPTIONS"},
		{"Access-Control-Allow-Headers", "Content-Type, X-API-Key"},
	}

	for _, h := range headers {
		got := rr.Header().Get(h.key)
		if got != h.val {
			t.Errorf("expected header %s to be %s, got %s", h.key, h.val, got)
		}
	}
}

func TestRateLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router := RateLimit(handler)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rr.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected rate limited status %d, got %d", http.StatusTooManyRequests, rr.Code)
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	limiterStore.mu.Lock()
	limiterStore.ips = make(map[string]*limiterEntry)
	limiterStore.mu.Unlock()

	limiterStore.getLimiter("1.1.1.1")
	limiterStore.getLimiter("2.2.2.2")

	limiterStore.mu.Lock()
	if len(limiterStore.ips) != 2 {
		t.Errorf("expected 2 limiters, got %d", len(limiterStore.ips))
	}
	limiterStore.ips["1.1.1.1"].lastActive = time.Now().Add(-20 * time.Minute)
	limiterStore.mu.Unlock()

	limiterStore.cleanup()

	limiterStore.mu.Lock()
	defer limiterStore.mu.Unlock()

	if len(limiterStore.ips) != 1 {
		t.Errorf("expected 1 limiter after cleanup, got %d", len(limiterStore.ips))
	}
	if _, exists := limiterStore.ips["2.2.2.2"]; !exists {
		t.Error("expected active limiter for 2.2.2.2 to remain")
	}
}
