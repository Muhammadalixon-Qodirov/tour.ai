package api

import (
	"math"
	"strconv"
	"sync"
	"time"
)

// Limiter — xotiradagi token bucket. Bitta server nusxasi uchun yetarli;
// bir nechta nusxa bo'lsa Redis ga ko'chiriladi.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewLimiter() *Limiter {
	l := &Limiter{buckets: map[string]*bucket{}, now: time.Now}
	go l.gc()
	return l
}

// Allow — so'rovga ruxsat; rad etilsa Retry-After (soniya) qaytaradi.
func (l *Limiter) Allow(key string, perMin int) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	rate := float64(perMin) / 60
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(perMin), last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(float64(perMin), b.tokens+now.Sub(b.last).Seconds()*rate)
	b.last = now
	if b.tokens < 1 {
		return strconv.Itoa(int(math.Ceil((1 - b.tokens) / rate))), false
	}
	b.tokens--
	return "", true
}

func (l *Limiter) gc() {
	for range time.Tick(5 * time.Minute) {
		l.mu.Lock()
		cutoff := l.now().Add(-10 * time.Minute)
		for k, b := range l.buckets {
			if b.last.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}
