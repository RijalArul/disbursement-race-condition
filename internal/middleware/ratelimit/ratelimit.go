// Package ratelimit implements an in-memory token-bucket limiter (BONUS-02).
// Bucket key is the JWT user_id for authenticated routes, never IP; the
// pre-auth /auth/login endpoint has no JWT yet, so it keys on IP and
// username instead (see ratelimit_rules.go).
package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
)

const (
	cleanupInterval = 10 * time.Minute
	idleEviction    = 10 * time.Minute
)

// bucket is a continuously-refilling token bucket. tokens and lastRefill are
// guarded by mu instead of an outer lock so buckets in the map can be updated
// independently of map structure changes.
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

// limiter holds one family of buckets (e.g. "per-user default limit") sharing
// a single rate/window, cleaned up on its own ticker.
type limiter struct {
	buckets sync.Map // string -> *bucket
	limit   int
	window  time.Duration
}

func newLimiter(limit int, window time.Duration) *limiter {
	l := &limiter{limit: limit, window: window}
	go l.cleanupLoop()
	return l
}

// cleanupLoop periodically evicts buckets idle longer than idleEviction so
// the map doesn't grow unbounded under key churn (e.g. many distinct usernames).
func (l *limiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.buckets.Range(func(key, value any) bool {
			b := value.(*bucket)
			b.mu.Lock()
			idle := now.Sub(b.lastSeen)
			b.mu.Unlock()
			if idle > idleEviction {
				l.buckets.Delete(key)
			}
			return true
		})
	}
}

// allow reports whether key has a token available, consuming one if so, and
// how long the caller should wait before retrying if not.
func (l *limiter) allow(key string) (ok bool, retryAfter time.Duration) {
	ref, _ := l.buckets.LoadOrStore(key, &bucket{tokens: float64(l.limit), lastRefill: time.Now(), lastSeen: time.Now()})
	b := ref.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.lastSeen = now

	refillRate := float64(l.limit) / l.window.Seconds()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * refillRate
	if b.tokens > float64(l.limit) {
		b.tokens = float64(l.limit)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		deficit := 1 - b.tokens
		return false, time.Duration(deficit/refillRate*1000) * time.Millisecond
	}

	b.tokens--
	return true, 0
}

// reject writes 429 with Retry-After and aborts the chain.
func reject(c *gin.Context, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", fmt.Sprintf("%d", seconds))
	response.Err(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
	c.Abort()
}

// PerUser rate-limits by the authenticated caller's user_id. Must be mounted
// after auth middleware so identity is already on context.
func PerUser(limit int, window time.Duration) gin.HandlerFunc {
	l := newLimiter(limit, window)
	return func(c *gin.Context) {
		key, ok := userKey(c)
		if !ok {
			c.Next()
			return
		}
		if allowed, retryAfter := l.allow(key); !allowed {
			reject(c, retryAfter)
			return
		}
		c.Next()
	}
}

// PerLoginAttempt dual-key rate-limits POST /auth/login: an IP bucket guards
// against noisy scanners, a username bucket guards against distributed
// credential-stuffing against one account regardless of source IP.
func PerLoginAttempt(ipLimit, usernameLimit int, window time.Duration) gin.HandlerFunc {
	ipLimiter := newLimiter(ipLimit, window)
	userLimiter := newLimiter(usernameLimit, window)
	return func(c *gin.Context) {
		if allowed, retryAfter := ipLimiter.allow(ipKey(c)); !allowed {
			reject(c, retryAfter)
			return
		}
		if key, ok := usernameKey(c); ok {
			if allowed, retryAfter := userLimiter.allow(key); !allowed {
				reject(c, retryAfter)
				return
			}
		}
		c.Next()
	}
}
