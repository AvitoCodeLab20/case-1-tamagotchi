package httpserver

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Defaults for the credential endpoints. A person signing in needs a handful of
// attempts; a script guessing passwords needs thousands.
const (
	defaultAuthRateBurst  = 10
	defaultAuthRateWindow = time.Minute

	// rateLimiterIdleTTL is how long an untouched bucket is kept before the
	// sweep drops it, so the map cannot grow without bound.
	rateLimiterIdleTTL = 10 * time.Minute
)

// bucket is one caller's token bucket.
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// rateLimiter is an in-memory token bucket per caller.
//
// In-memory means the budget is per process: behind several replicas an
// attacker gets the limit times the number of replicas. That is acceptable for
// the MVP and the reason the limiter is a separate type — swapping the storage
// for Redis is a change in one place.
type rateLimiter struct {
	mutex     sync.Mutex
	buckets   map[string]*bucket
	burst     float64
	refillPer float64 // tokens per second
	lastSweep time.Time
	now       func() time.Time
}

// newRateLimiter allows burst requests, refilled evenly over window.
func newRateLimiter(burst int, window time.Duration) *rateLimiter {
	now := time.Now()

	return &rateLimiter{
		buckets:   make(map[string]*bucket),
		burst:     float64(burst),
		refillPer: float64(burst) / window.Seconds(),
		lastSweep: now,
		now:       time.Now,
	}
}

// allow takes one token from the caller's bucket and reports whether it was
// available, along with how long to wait when it was not.
func (limiter *rateLimiter) allow(key string) (bool, time.Duration) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	limiter.sweep(now)

	current, found := limiter.buckets[key]
	if !found {
		current = &bucket{tokens: limiter.burst, lastRefill: now}
		limiter.buckets[key] = current
	}

	current.tokens = math.Min(limiter.burst, current.tokens+now.Sub(current.lastRefill).Seconds()*limiter.refillPer)
	current.lastRefill = now

	if current.tokens < 1 {
		retryAfter := time.Duration((1 - current.tokens) / limiter.refillPer * float64(time.Second))

		return false, retryAfter
	}

	current.tokens--

	return true, 0
}

// sweep drops buckets nobody has touched recently. The caller holds the mutex.
func (limiter *rateLimiter) sweep(now time.Time) {
	if now.Sub(limiter.lastSweep) < rateLimiterIdleTTL {
		return
	}
	limiter.lastSweep = now

	for key, current := range limiter.buckets {
		if now.Sub(current.lastRefill) >= rateLimiterIdleTTL {
			delete(limiter.buckets, key)
		}
	}
}

// throttle rejects a caller that exceeded its request budget.
func throttle(limiter *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			allowed, retryAfter := limiter.allow(clientIP(request))
			if !allowed {
				response.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				writeError(response, http.StatusTooManyRequests, codeRateLimited, "too many requests, please try again later")

				return
			}

			next.ServeHTTP(response, request)
		})
	}
}

// clientIP derives the limiter key from the transport address.
//
// Forwarding headers are deliberately ignored: any client can set
// X-Forwarded-For, so trusting it would let an attacker get a fresh budget per
// request. Once the service runs behind a known proxy, that proxy's header
// becomes trustworthy and this is where it gets read.
func clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}

	return host
}
