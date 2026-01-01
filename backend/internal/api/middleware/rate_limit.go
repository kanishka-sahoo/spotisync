package middleware

import (
	"math"
	"net/http"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
	// CleanupInterval is how often to remove stale entries (default: 5 minutes)
	CleanupInterval time.Duration
	// StaleThreshold is how old an entry must be to be considered stale (default: 15 minutes)
	StaleThreshold time.Duration
}

// TokenBucket implements a simple token bucket rate limiter
type TokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a new token bucket with the specified max tokens and refill rate
func NewTokenBucket(maxTokens, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed based on available tokens
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.lastRefill = now

	// Add tokens based on elapsed time
	tb.tokens = math.Min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// IPRateLimiter implements per-IP rate limiting using token buckets
type IPRateLimiter struct {
	buckets         sync.Map // map[string]*TokenBucket
	refillRate      float64
	maxTokens       float64
	cleanupInterval time.Duration
	staleThreshold  time.Duration
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewIPRateLimiter creates a new per-IP rate limiter
func NewIPRateLimiter(requestsPerMinute, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		refillRate:      float64(requestsPerMinute) / 60.0,
		maxTokens:       float64(burst),
		cleanupInterval: 5 * time.Minute,
		staleThreshold:  15 * time.Minute,
		stopChan:        make(chan struct{}),
	}
}

// StartCleanupRoutine starts the background cleanup routine for stale entries
func (rl *IPRateLimiter) StartCleanupRoutine() {
	rl.wg.Add(1)
	go func() {
		defer rl.wg.Done()
		ticker := time.NewTicker(rl.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-rl.stopChan:
				return
			case <-ticker.C:
				rl.cleanupStaleEntries()
			}
		}
	}()
}

// StopCleanupRoutine stops the cleanup routine
func (rl *IPRateLimiter) StopCleanupRoutine() {
	close(rl.stopChan)
	rl.wg.Wait()
}

// cleanupStaleEntries removes rate limiters that haven't been used recently
func (rl *IPRateLimiter) cleanupStaleEntries() {
	now := time.Now()
	rl.buckets.Range(func(key, value interface{}) bool {
		ip := key.(string)
		tb := value.(*TokenBucket)

		tb.mu.Lock()
		lastRefill := tb.lastRefill
		tb.mu.Unlock()

		if now.Sub(lastRefill) > rl.staleThreshold {
			rl.buckets.Delete(ip)
		}

		return true
	})
}

// getBucket gets or creates a token bucket for the given IP
func (rl *IPRateLimiter) getBucket(ip string) *TokenBucket {
	if tb, ok := rl.buckets.Load(ip); ok {
		return tb.(*TokenBucket)
	}

	// Create a new bucket
	tb := NewTokenBucket(rl.maxTokens, rl.refillRate)

	// Store it atomically
	actual, loaded := rl.buckets.LoadOrStore(ip, tb)
	if loaded {
		// Another goroutine created it first, use that one
		return actual.(*TokenBucket)
	}

	return tb
}

// Allow checks if a request from the given IP should be allowed
func (rl *IPRateLimiter) Allow(ip string) bool {
	tb := rl.getBucket(ip)
	return tb.Allow()
}

// RateLimitMiddleware creates rate limiting middleware using per-IP token bucket algorithm
func RateLimitMiddleware(requestsPerMinute, burst int) func(http.Handler) http.Handler {
	rl := NewIPRateLimiter(requestsPerMinute, burst)
	rl.StartCleanupRoutine()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				// Take the first IP in the chain
				ip = forwarded
			}

			if !rl.Allow(ip) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
