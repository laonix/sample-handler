package http

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	net_url "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRateLimit     = 999
	defaultLimitDuration = time.Second
)

// resSizes holds a slice of byte lengths of responses bodies.
type resSizes struct {
	mu sync.Mutex
	s  []int
}

// String returns a string representation of resSizes:
// strings with responses bodies lengths in bytes separated by a new line.
func (rs *resSizes) String() string {
	b := strings.Builder{}

	sLen := len(rs.s)
	for i, size := range rs.s {
		b.WriteString(strconv.Itoa(size))
		if sLen-i > 1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// Add appends a byte length of response body to resSizes.
func (rs *resSizes) Add(i int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.s = append(rs.s, i)
}

// Getter is a contract for performing HTTP GET requests.
//
// Standart http.Client satisfies Getter interface.
type Getter interface {
	Get(url string) (resp *http.Response, err error)
}

// ResponseSizeCounter is an implementation of http.Handler.
type ResponseSizeCounter struct {
	urls  []string
	sizes resSizes

	client Getter
}

// MakeResponseSizeCounter returns a new instance of ResponseSizeCounter wrapped in RateLimit middleware.
func MakeResponseSizeCounter() http.Handler {
	rateLimitMW := RateLimit(defaultRateLimit, defaultLimitDuration, NewStatHolder())
	rsc := &ResponseSizeCounter{
		client: http.DefaultClient,
		sizes:  resSizes{},
	}
	return rateLimitMW(rsc)
}

// ServeHTTP receives a POST request with urls separated by a new line,
// performs GET requests to each of that urls and returns within its response
// a string of new-line separated byte lengths of performed requests responses.
func (h *ResponseSizeCounter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// I'd rather use github.com/gorilla/handlers and github.com/gorilla/mux
	// to manage middleware and methods to handlers mapping,
	// but here we go
	if req.Method == http.MethodPost {
		h.serve(w, req)
	} else {
		http.Error(w, "Only POST method supported.", http.StatusMethodNotAllowed)
		return
	}
}
func (h *ResponseSizeCounter) serve(w http.ResponseWriter, req *http.Request) {
	if err := h.getUrls(req); err != nil {
		http.Error(w, fmt.Errorf("get urls: %s", err).Error(), http.StatusInternalServerError)
		return
	}

	if err := h.getRespSizes(); err != nil {
		http.Error(w, fmt.Errorf("get sizes of responses: %s", err).Error(), http.StatusInternalServerError)
		return
	}

	_, err := w.Write([]byte(h.sizes.String()))
	if err != nil {
		http.Error(w, fmt.Errorf("write response: %s", err).Error(), http.StatusInternalServerError)
		return
	}
}

func (h *ResponseSizeCounter) getUrls(req *http.Request) error {
	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("read request body: %s", err)
	}

	lines, err := splitToLines(string(bytes))
	if err != nil {
		return fmt.Errorf("split request body to lines: %s", err)
	}

	h.urls = make([]string, 0)
	for _, line := range lines {
		line := line
		if isUrl(line) {
			h.urls = append(h.urls, line)
		} else {
			return errors.New(fmt.Sprintf("'%s' is not a URL", line))
		}
	}

	return nil
}

func (h *ResponseSizeCounter) getRespSizes() error {
	h.sizes.s = make([]int, 0)

	// I'd rather use errgroup.Group of golang.org/x/sync/errgroup package,
	// but here we go
	var wg sync.WaitGroup
	var errOnce sync.Once
	var err error

	for _, url := range h.urls {
		wg.Add(1)

		url := url
		go func(single error) {
			defer wg.Done()

			size, err := h.doGet(url)
			if err != nil {
				errOnce.Do(func() {
					single = err
				})
			}

			h.sizes.Add(size)
		}(err)
	}

	wg.Wait()

	return err
}

func (h *ResponseSizeCounter) doGet(url string) (size int, err error) {
	res, err := h.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("GET '%s': %s", url, err)
	}
	defer closeResBody(res.Body)

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf("read response body: %s", err)
	}

	return len(bytes), err
}

func splitToLines(input string) (lines []string, err error) {
	lines = make([]string, 0)
	sc := bufio.NewScanner(strings.NewReader(input))

	for sc.Scan() {
		line := sc.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines, sc.Err()
}

func isUrl(str string) bool {
	u, err := net_url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func closeResBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.Printf("close response body: %s", err)
	}
}
