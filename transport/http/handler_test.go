package http

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"

	http_mock "github.com/laonix/sample-handler/transport/http/mock"
)

//go:generate mockgen -destination mock/client_mock.go -package http_mock -mock_names Getter=MockClient github.com/laonix/sample-handler/transport/http Getter

func TestResponseSizeCounter_ServeHTTP_happyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := http_mock.NewMockClient(ctrl)
	{
		calls := make([]*gomock.Call, 0)
		for i := 0; i < 3; i++ {
			calls = append(calls, client.EXPECT().Get(gomock.Any()).Return(response(http.StatusOK), nil))
		}
		gomock.InOrder(calls...)
	}

	handler := &ResponseSizeCounter{
		client: client,
		sizes:  resSizes{},
	}

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, request())

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("operation failed")
	}
	defer closeResBody(res.Body)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("cannot read response body: %s", err)
	}

	resLines, err := splitToLines(string(body))
	if err != nil {
		t.Errorf("cannot split response body to lines: %s", err)
	}

	if len(resLines) != 3 {
		t.Errorf("response lines count: want = %d, got = %d", 3, len(resLines))
	}

	for _, line := range resLines {
		if line != "25000" {
			t.Errorf("wrong response size: want = %s, got = %s", "25000", line)
		}
	}
}

func TestResponseSizeCounter_ServeHTTP_wrongMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := http_mock.NewMockClient(ctrl)

	handler := &ResponseSizeCounter{
		client: client,
		sizes:  resSizes{},
	}

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, &http.Request{Method: http.MethodGet})

	res := w.Result()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Error("not allowed request method handled incorrectly")
	}
	defer closeResBody(res.Body)
}

func TestResponseSizeCounter_ServeHTTP_wrongInput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := http_mock.NewMockClient(ctrl)

	handler := &ResponseSizeCounter{
		client: client,
		sizes:  resSizes{},
	}

	w := httptest.NewRecorder()

	handler.ServeHTTP(w, badRequest())

	res := w.Result()
	if res.StatusCode != http.StatusInternalServerError {
		t.Error("wrong request body method handled incorrectly")
	}
	defer closeResBody(res.Body)
}

func request() *http.Request {
	body := `https://test-1.com
http://test-2.com
https://test-3.com`

	return &http.Request{
		Method: http.MethodPost,
		Body:   io.NopCloser(bytes.NewBufferString(body)),
	}
}

func badRequest() *http.Request {
	body := `test-1.xyz
test-2.123
test-3.abc`

	return &http.Request{
		Method: http.MethodPost,
		Body:   io.NopCloser(bytes.NewBufferString(body)),
	}
}

func response(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("0", 25*1000))), // body of size 25 kb
	}
}
