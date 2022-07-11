package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type Stat interface {
	Reset()
	Increment(id string) int32
}

type StatHolder struct {
	mu      sync.RWMutex
	counter map[string]int32
}

func NewStatHolder() *StatHolder {
	return &StatHolder{
		counter: make(map[string]int32),
	}
}

func (sh *StatHolder) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.counter = make(map[string]int32)
}

func (sh *StatHolder) Increment(id string) int32 {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.counter[id]++

	return sh.counter[id]
}

func RateLimit(limit int, window time.Duration, stat Stat) func(next http.Handler) http.Handler {
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
