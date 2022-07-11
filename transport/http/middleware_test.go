package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	http_mock "github.com/laonix/sample-handler/transport/http/mock"
)

//go:generate mockgen -destination mock/handler_mock.go -package http_mock net/http Handler

func TestRateLimit_tooManyRequests(t *testing.T) {
	rl := RateLimit(3, time.Second, NewStatHolder())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := http_mock.NewMockHandler(ctrl)
	{
		h.EXPECT().ServeHTTP(gomock.Any(), gomock.Any()).AnyTimes()
	}

	w := httptest.NewRecorder()

	for i := 0; i < 4; i++ {
		rl(h).ServeHTTP(w, requestWithIP("127.0.0.1:80"))
	}

	res := w.Result()
	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Wrong response status: want = %d, got = %d", http.StatusTooManyRequests, res.StatusCode)
	}
}

func TestRateLimit_wrongRequestIP(t *testing.T) {
	rl := RateLimit(3, time.Second, NewStatHolder())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := http_mock.NewMockHandler(ctrl)
	{
		h.EXPECT().ServeHTTP(gomock.Any(), gomock.Any()).AnyTimes()
	}

	w := httptest.NewRecorder()

	rl(h).ServeHTTP(w, requestWithIP("0.0.0.0"))

	res := w.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Wrong response status: want = %d, got = %d", http.StatusBadRequest, res.StatusCode)
	}
}

func requestWithIP(ip string) *http.Request {
	return &http.Request{
		Method:     http.MethodGet,
		RemoteAddr: ip,
	}
}
