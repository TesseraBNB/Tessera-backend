package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "requestID"

// middleware composes the request pipeline (outermost first):
// recover → request-id → log → CORS → rate-limit.
func (a *App) middleware(next http.Handler) http.Handler {
	return a.recoverer(a.requestID(a.logger(a.cors(a.rateLimit(next)))))
}

func (a *App) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.log.Error("panic recovered", "err", rec, "path", r.URL.Path)
				a.jsonError(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := randID()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func (a *App) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		a.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur", time.Since(start).String(),
			"id", r.Context().Value(requestIDKey),
		)
	})
}

func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (slices.Contains(a.cfg.AllowedOrigins, origin) || slices.Contains(a.cfg.AllowedOrigins, "*")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.limiter.allow(clientIP(r)) {
			a.jsonError(w, "rate limit exceeded — slow down", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the response status and preserves http.Flusher so SSE
// keeps working through the middleware chain.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.wrote = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- per-IP token-bucket rate limiter ---

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      float64
	burst    float64
}

type visitor struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	rl := &rateLimiter{visitors: make(map[string]*visitor), rps: rps, burst: float64(burst)}
	go rl.cleanupLoop()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	v, ok := rl.visitors[ip]
	if !ok {
		rl.visitors[ip] = &visitor{tokens: rl.burst - 1, last: now}
		return true
	}
	v.tokens += rl.rps * now.Sub(v.last).Seconds()
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.last = now
	if v.tokens >= 1 {
		v.tokens--
		return true
	}
	return false
}

// cleanupLoop evicts idle visitors so the map does not grow unbounded.
func (rl *rateLimiter) cleanupLoop() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.last) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// --- global daily cost guard for agent runs ---

type dailyBudget struct {
	mu    sync.Mutex
	limit int
	day   string
	count int
}

func newDailyBudget(limit int) *dailyBudget { return &dailyBudget{limit: limit} }

// allow reports whether another agent run is permitted today (limit <= 0 means
// unlimited) and records it.
func (b *dailyBudget) allow() bool {
	if b.limit <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	if today != b.day {
		b.day = today
		b.count = 0
	}
	if b.count >= b.limit {
		return false
	}
	b.count++
	return true
}
