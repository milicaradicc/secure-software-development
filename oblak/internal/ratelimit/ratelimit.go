package ratelimit

import (
    "net/http"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

type Limiter struct {
    mu       sync.Mutex
    calls    map[string][]time.Time
    maxCalls int
    window   time.Duration
}

func New(maxCalls int, window time.Duration) *Limiter {
    return &Limiter{
        calls:    make(map[string][]time.Time),
        maxCalls: maxCalls,
        window:   window,
    }
}

func (l *Limiter) Check(key string) bool {
    l.mu.Lock()
    defer l.mu.Unlock()

    now := time.Now()
    windowStart := now.Add(-l.window)

    // Ukloni stare pozive
    valid := l.calls[key][:0]
    for _, t := range l.calls[key] {
        if t.After(windowStart) {
            valid = append(valid, t)
        }
    }
    l.calls[key] = valid

    if len(l.calls[key]) >= l.maxCalls {
        return false
    }

    l.calls[key] = append(l.calls[key], now)
    return true
}

// Gin middleware
func (l *Limiter) Middleware(keyFunc func(*gin.Context) string) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := keyFunc(c)
        if !l.Check(key) {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "detail": "Rate limit prekoračen",
            })
            return
        }
        c.Next()
    }
}

var (
    LoginLimiter  = New(5, time.Minute)
    DeployLimiter = New(10, time.Hour)
    InvokeLimiter = New(30, time.Minute)
)