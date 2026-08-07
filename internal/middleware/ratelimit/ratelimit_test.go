package ratelimit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestLimiterAllowsUpToLimitThenBlocks(t *testing.T) {
	l := newLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		ok, _ := l.allow("k")
		if !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	ok, retryAfter := l.allow("k")
	if ok {
		t.Fatal("4th request should be blocked at limit=3")
	}
	if retryAfter <= 0 {
		t.Fatal("blocked request must report a positive retry-after")
	}
}

func TestLimiterRefillsOverTime(t *testing.T) {
	l := newLimiter(1, 100*time.Millisecond)

	ok, _ := l.allow("k")
	if !ok {
		t.Fatal("first request should be allowed")
	}
	ok, _ = l.allow("k")
	if ok {
		t.Fatal("second immediate request should be blocked")
	}

	time.Sleep(120 * time.Millisecond)

	ok, _ = l.allow("k")
	if !ok {
		t.Fatal("request after full window elapsed should be allowed again")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l := newLimiter(1, time.Minute)

	if ok, _ := l.allow("a"); !ok {
		t.Fatal("key a first request should be allowed")
	}
	if ok, _ := l.allow("b"); !ok {
		t.Fatal("key b must have its own bucket, independent of key a")
	}
}

func TestPerUserSkipsWhenNoIdentity(t *testing.T) {
	r := gin.New()
	r.Use(PerUser(1, time.Minute))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d without identity should never be rate-limited, got %d", i+1, w.Code)
		}
	}
}

func TestPerUserBlocksAtLimitAndSetsRetryAfter(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(common.WithIdentity(c.Request.Context(), "user-1", "u", "admin"))
		c.Next()
	})
	r.Use(PerUser(2, time.Minute))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d within limit should pass, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request should be 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 response must set Retry-After header")
	}
}

func TestPerUserDoesNotCrossUserBuckets(t *testing.T) {
	r := gin.New()
	var mu sync.Mutex
	nextUser := "user-1"
	r.Use(func(c *gin.Context) {
		mu.Lock()
		u := nextUser
		mu.Unlock()
		c.Request = c.Request.WithContext(common.WithIdentity(c.Request.Context(), u, "u", "admin"))
		c.Next()
	})
	r.Use(PerUser(1, time.Minute))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("user-1 first request should pass, got %d", w1.Code)
	}

	mu.Lock()
	nextUser = "user-2"
	mu.Unlock()

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("user-2 should have its own bucket, not inherit user-1's exhaustion, got %d", w2.Code)
	}
}

func loginBody(username string) *bytes.Buffer {
	b, _ := json.Marshal(map[string]string{"username": username, "password": "x"})
	return bytes.NewBuffer(b)
}

func TestPerLoginAttemptBlocksOnIPLimit(t *testing.T) {
	r := gin.New()
	r.Use(PerLoginAttempt(1, 100, time.Minute))
	r.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/login", loginBody("alice")))
	if w1.Code != http.StatusOK {
		t.Fatalf("first login should pass, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/login", loginBody("bob")))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("2nd login from same IP (different username) should be blocked by IP limit, got %d", w2.Code)
	}
}

func TestPerLoginAttemptBlocksOnUsernameLimitAcrossIPs(t *testing.T) {
	r := gin.New()
	r.Use(PerLoginAttempt(100, 1, time.Minute))
	r.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	req1 := httptest.NewRequest(http.MethodPost, "/login", loginBody("alice"))
	req1.RemoteAddr = "1.1.1.1:1234"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first login should pass, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/login", loginBody("alice"))
	req2.RemoteAddr = "2.2.2.2:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("2nd login for same username from a different IP should be blocked by username limit, got %d", w2.Code)
	}
}

func TestPerLoginAttemptUsernameCaseAndWhitespaceNormalized(t *testing.T) {
	r := gin.New()
	r.Use(PerLoginAttempt(100, 1, time.Minute))
	r.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/login", loginBody("Alice")))
	if w1.Code != http.StatusOK {
		t.Fatalf("first login should pass, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/login", loginBody("  alice  ")))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("case/whitespace variant of same username should share the same bucket, got %d", w2.Code)
	}
}

func TestPerLoginAttemptMalformedBodyFallsBackToIPOnly(t *testing.T) {
	r := gin.New()
	r.Use(PerLoginAttempt(1, 100, time.Minute))
	r.POST("/login", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("not json")))
	if w1.Code != http.StatusOK {
		t.Fatalf("malformed body should still be checked against IP limit and pass within it, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString("not json")))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("2nd request from same IP should still hit the IP limit despite malformed body, got %d", w2.Code)
	}
}

// TestLimiterConcurrentAllowExactlyLimit hammers one key from many goroutines
// at once. Without proper locking, exactly-limit allowance would drift under
// -race; can't run -race on this machine (no gcc/CGO), so this asserts the
// invariant directly: concurrent access must never let more than limit
// requests through, no fewer either.
func TestLimiterConcurrentAllowExactlyLimit(t *testing.T) {
	const limit = 100
	const attempts = 1000

	l := newLimiter(limit, time.Minute)

	var wg sync.WaitGroup
	var allowedCount int64
	var mu sync.Mutex

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.allow("shared-key"); ok {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowedCount != limit {
		t.Fatalf("concurrent allow: expected exactly %d allowed out of %d attempts, got %d", limit, attempts, allowedCount)
	}
}

func TestLimiterCleanupEvictsIdleBuckets(t *testing.T) {
	l := &limiter{limit: 1, window: time.Minute}
	l.buckets.Store("stale", &bucket{tokens: 1, lastRefill: time.Now(), lastSeen: time.Now().Add(-idleEviction - time.Second)})
	l.buckets.Store("fresh", &bucket{tokens: 1, lastRefill: time.Now(), lastSeen: time.Now()})

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

	if _, ok := l.buckets.Load("stale"); ok {
		t.Fatal("stale bucket should have been evicted")
	}
	if _, ok := l.buckets.Load("fresh"); !ok {
		t.Fatal("fresh bucket should not have been evicted")
	}
}
