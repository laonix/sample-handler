package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Stat is a contract for handling requests rate statistics.
type Stat interface {
	// Reset performs statistics flushing.
	//
	// Reset is supposed to be called after each agreed time interval.
	Reset()

	// Increment increases a counter of requests from a given IP by 1.
	//
	// This operation collects statistics for each IP to decide if a rate limit is exceeded.
	Increment(id string) int32
}

// StatHolder is a default implementation of Stat.
type StatHolder struct {
	mu      sync.RWMutex
	counter map[string]int32
}

// NewStatHolder returns a new instance of StatHolder.
func NewStatHolder() *StatHolder {
	return &StatHolder{
		counter: make(map[string]int32),
	}
}

// Reset clears underlying counter map.
func (sh *StatHolder) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.counter = make(map[string]int32)
}

// Increment adds 1 to a counter of requests incoming from a given IP.
func (sh *StatHolder) Increment(id string) int32 {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.counter[id]++

	return sh.counter[id]
}

// RateLimit creates a middleware wrapping a given handler.
// It allows to set a rate limit for requests from each IP at a certain time window.
func RateLimit(limit int, window time.Duration, stat Stat) func(next http.Handler) http.Handler {
	// I'd rather use Limiter from golang.org/x/time/rate package,
	// but here we go
	ticker := time.NewTicker(window)
	go func() {
		for range ticker.C {
			stat.Reset()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			reqIP, err := requestIP(req)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}

			current := int(stat.Increment(reqIP))

			if limit < current {
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}

func requestIP(req *http.Request) (string, error) {
	// in real production we should check X-REAL-IP, X-FORWARDED-FOR... request headers
	// to prevent the case when client is behind proxy, uses load balancer or so
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return "", err
	}

	return ip, nil
}
