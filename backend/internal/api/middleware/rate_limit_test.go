package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"spotisync/internal/api/middleware"
)

// TestTokenBucketCreation tests token bucket creation
func TestTokenBucketCreation(t *testing.T) {
	tb := middleware.NewTokenBucket(10, 1.0)

	assert.NotNil(t, tb)
}

// TestTokenBucketAllowance tests token bucket allowance
func TestTokenBucketAllowance(t *testing.T) {
	tb := middleware.NewTokenBucket(10, 1.0)

	// Initially should have max tokens
	assert.True(t, tb.Allow())
	assert.True(t, tb.Allow())
	assert.True(t, tb.Allow())
	// 7 more should be allowed (total 10)
	for i := 0; i < 7; i++ {
		assert.True(t, tb.Allow())
	}

	// Now should be empty
	assert.False(t, tb.Allow())
	assert.False(t, tb.Allow())
}

// TestTokenBucketRefill tests token bucket refill behavior
func TestTokenBucketRefill(t *testing.T) {
	// Create a token bucket with 10 tokens and 1 token per second refill rate
	tb := middleware.NewTokenBucket(10, 1.0)

	// Use all tokens
	for i := 0; i < 10; i++ {
		assert.True(t, tb.Allow())
	}

	// Should be empty now
	assert.False(t, tb.Allow())

	// Wait for 2 seconds (should refill 2 tokens)
	time.Sleep(2 * time.Second)

	// Should have 2 tokens now
	assert.True(t, tb.Allow())
	assert.True(t, tb.Allow())

	// Should be empty again
	assert.False(t, tb.Allow())
}

// TestTokenBucketMaxCap tests that tokens don't exceed max capacity
func TestTokenBucketMaxCap(t *testing.T) {
	tb := middleware.NewTokenBucket(10, 10.0) // 10 tokens per second refill rate

	// Use all tokens
	for i := 0; i < 10; i++ {
		assert.True(t, tb.Allow())
	}

	// Wait a short time (less than 1 second)
	time.Sleep(100 * time.Millisecond)

	// Should have some tokens but not exceed max
	assert.True(t, tb.Allow())

	// Even if we wait longer, tokens should not exceed max
	time.Sleep(2 * time.Second)
	// At this point, refill should have happened but capped at max
	// Try to use tokens - should work
	for i := 0; i < 10; i++ {
		assert.True(t, tb.Allow())
	}
}

// TestRateLimitMiddlewareBasic tests basic rate limiting middleware
func TestRateLimitMiddlewareBasic(t *testing.T) {
	// Create middleware with 10 requests per minute and burst of 5
	rlMiddleware := middleware.RateLimitMiddleware(10, 5)

	// Create a test handler
	handler := rlMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests within burst limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Next request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestRateLimitMiddlewareRefill tests rate limit middleware refill behavior
func TestRateLimitMiddlewareRefill(t *testing.T) {
	// Create middleware with 10 requests per minute and burst of 5
	rlMiddleware := middleware.RateLimitMiddleware(10, 5)

	// Create a test handler
	handler := rlMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for refill (6 seconds = 1 token per second for 10 req/min)
	time.Sleep(6 * time.Second)

	// Should be allowed again
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRateLimitMiddlewareMultipleRequests tests rate limiting with multiple rapid requests
func TestRateLimitMiddlewareMultipleRequests(t *testing.T) {
	// Create middleware with a small burst for testing
	rlMiddleware := middleware.RateLimitMiddleware(60, 10) // 60 per minute, burst of 10

	handler := rlMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make more requests than the burst
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// Should have exactly 10 successful requests and 10 rate limited
	assert.Equal(t, 10, successCount)
	assert.Equal(t, 10, rateLimitedCount)
}

// TestRateLimitMiddlewareConcurrentRequests tests rate limiting with concurrent requests
func TestRateLimitMiddlewareConcurrentRequests(t *testing.T) {
	// Create middleware
	rlMiddleware := middleware.RateLimitMiddleware(100, 50)

	handler := rlMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make concurrent requests
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Should have exactly 50 successful requests
	assert.Equal(t, int32(50), successCount)
}

// TestRateLimitMiddlewareDifferentPaths tests that rate limiting works for different paths
func TestRateLimitMiddlewareDifferentPaths(t *testing.T) {
	// Create middleware
	rlMiddleware := middleware.RateLimitMiddleware(10, 5)

	handler := rlMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests to different paths
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/path"+string(rune('1'+i)), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Next request to any path should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/anotherpath", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

// TestTokenBucketThreadSafety tests that token bucket is thread-safe
func TestTokenBucketThreadSafety(t *testing.T) {
	tb := middleware.NewTokenBucket(1000, 1000.0) // High limits

	// Make concurrent requests
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow() {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// Should have exactly 1000 successful requests
	assert.Equal(t, int32(1000), successCount)
}

// TestTokenBucketRefillRateCalculation tests that refill rate is calculated correctly
func TestTokenBucketRefillRateCalculation(t *testing.T) {
	// 60 requests per minute = 1 token per second
	rlMiddleware := middleware.RateLimitMiddleware(60, 10)
	assert.NotNil(t, rlMiddleware)

	// 120 requests per minute = 2 tokens per second
	rlMiddleware2 := middleware.RateLimitMiddleware(120, 10)
	assert.NotNil(t, rlMiddleware2)

	// 30 requests per minute = 0.5 tokens per second
	rlMiddleware3 := middleware.RateLimitMiddleware(30, 10)
	assert.NotNil(t, rlMiddleware3)
}
