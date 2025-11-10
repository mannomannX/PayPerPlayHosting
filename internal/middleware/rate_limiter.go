package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
	rate     time.Duration
	burst    int
}

type Visitor struct {
	tokens     int
	lastSeen   time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
// rate: time between token refills
// burst: maximum number of tokens
func NewRateLimiter(rate time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		rate:     rate,
		burst:    burst,
	}

	// Cleanup old visitors every 5 minutes
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	visitor, exists := rl.visitors[ip]
	if !exists {
		visitor = &Visitor{
			tokens:   rl.burst,
			lastSeen: time.Now(),
		}
		rl.visitors[ip] = visitor
	}
	rl.mu.Unlock()

	visitor.mu.Lock()
	defer visitor.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(visitor.lastSeen)
	tokensToAdd := int(elapsed / rl.rate)

	if tokensToAdd > 0 {
		visitor.tokens += tokensToAdd
		if visitor.tokens > rl.burst {
			visitor.tokens = rl.burst
		}
		visitor.lastSeen = now
	}

	// Check if we have tokens
	if visitor.tokens > 0 {
		visitor.tokens--
		return true
	}

	return false
}

// cleanup removes old visitors
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, visitor := range rl.visitors {
			visitor.mu.Lock()
			if time.Since(visitor.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
			visitor.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware creates a Gin middleware for rate limiting
func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"code":  "RATE_LIMIT_EXCEEDED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Different rate limiters for different endpoints
var (
	// Global rate limiter: 300 requests per minute (very lenient for GET requests)
	GlobalRateLimiter = NewRateLimiter(200*time.Millisecond, 300)

	// API rate limiter: 120 requests per minute (2 per second for normal API operations)
	APIRateLimiter = NewRateLimiter(500*time.Millisecond, 120)

	// Auth rate limiter: 5 requests per 15 seconds (strict for login/register to prevent brute force)
	AuthRateLimiter = NewRateLimiter(3*time.Second, 5)

	// File upload operations: 30 requests per minute (icons, resource packs, etc.)
	FileUploadRateLimiter = NewRateLimiter(2*time.Second, 30)

	// Expensive operations: 15 requests per minute (backups, restores, etc.)
	ExpensiveRateLimiter = NewRateLimiter(4*time.Second, 15)
)
