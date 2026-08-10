package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	limiter := newRateLimiter(3, time.Minute)

	for attempt := 1; attempt <= 3; attempt++ {
		if allowed, _ := limiter.allow("client"); !allowed {
			t.Fatalf("attempt %d was blocked, want the first 3 allowed", attempt)
		}
	}

	allowed, retryAfter := limiter.allow("client")
	if allowed {
		t.Error("the 4th attempt was allowed, want it blocked")
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter = %s, want a positive wait", retryAfter)
	}
}

func TestRateLimiterIsPerCaller(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)

	if allowed, _ := limiter.allow("first"); !allowed {
		t.Fatal("the first caller was blocked on its first attempt")
	}
	// A second caller must not pay for the first caller's budget.
	if allowed, _ := limiter.allow("second"); !allowed {
		t.Error("the second caller was blocked on its first attempt")
	}
	if allowed, _ := limiter.allow("first"); allowed {
		t.Error("the first caller was allowed a second attempt")
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter := newRateLimiter(2, time.Minute)

	now := time.Now()
	limiter.now = func() time.Time { return now }

	for range 2 {
		if allowed, _ := limiter.allow("client"); !allowed {
			t.Fatal("a burst attempt was blocked")
		}
	}
	if allowed, _ := limiter.allow("client"); allowed {
		t.Fatal("the budget was not exhausted")
	}

	// Half a window refills one of the two tokens.
	now = now.Add(30 * time.Second)

	if allowed, _ := limiter.allow("client"); !allowed {
		t.Error("no token was refilled after half a window")
	}
}

func TestRateLimiterSweepsIdleBuckets(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)

	now := time.Now()
	limiter.now = func() time.Time { return now }

	limiter.allow("client")

	if len(limiter.buckets) != 1 {
		t.Fatalf("buckets = %d, want 1", len(limiter.buckets))
	}

	now = now.Add(2 * rateLimiterIdleTTL)
	limiter.allow("another-client")

	// The idle bucket is gone, so a long-running process cannot be made to leak
	// memory by rotating source addresses.
	if _, found := limiter.buckets["client"]; found {
		t.Error("the idle bucket survived the sweep")
	}
}

func TestLoginEndpointIsThrottled(t *testing.T) {
	s := newSuite(t)

	var lastCode int

	// The burst is small, so a few more attempts than the burst must trip it.
	for range defaultAuthRateBurst + 1 {
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "203.0.113.7:5000"
		s.handler.ServeHTTP(response, request)
		lastCode = response.Code

		if lastCode == http.StatusTooManyRequests {
			if retryAfter := response.Header().Get("Retry-After"); retryAfter == "" {
				t.Error("Retry-After header is missing from the 429")
			} else if _, err := strconv.Atoi(retryAfter); err != nil {
				t.Errorf("Retry-After = %q, want a number of seconds", retryAfter)
			}

			return
		}
	}

	t.Errorf("last status = %d, want %d after exceeding the burst", lastCode, http.StatusTooManyRequests)
}

func TestUnthrottledEndpointStaysOpen(t *testing.T) {
	s := newSuite(t)

	// Health checks run on a timer; throttling them would take the service out
	// of the load balancer.
	for range defaultAuthRateBurst * 3 {
		response := s.request(t, http.MethodGet, "/healthz", nil, "")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
	}
}
