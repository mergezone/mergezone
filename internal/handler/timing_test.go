package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"merge/internal/provider"
)

var timingAttribute = regexp.MustCompile(`data-(gather|logic|template)-ms="([\d.]+)"`)

func TestRequestTimingMiddlewareAppendsBodyTiming(t *testing.T) {
	handler := requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordProviderTimings(r, provider.PullRequestTimings{
			Gather: 3 * time.Millisecond,
		})
		time.Sleep(3 * time.Millisecond)
		markRequestWorkComplete(r)
		time.Sleep(3 * time.Millisecond)
		_, _ = w.Write([]byte("page"))
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Header().Get("Server-Timing") != "" {
		t.Fatal("timings should be sent in the response body, not a header")
	}
	body := response.Body.String()
	if !strings.HasPrefix(body, "page<div hidden data-status-timings") {
		t.Fatalf("timing payload was not appended after page body: %q", body)
	}
	found := map[string]bool{}
	for _, match := range timingAttribute.FindAllStringSubmatch(body, -1) {
		milliseconds, err := strconv.ParseFloat(match[2], 64)
		if err != nil || milliseconds <= 0 {
			t.Errorf("%s duration = %q, want positive milliseconds", match[1], match[2])
		}
		found[match[1]] = true
	}
	if !found["gather"] || !found["logic"] || !found["template"] {
		t.Fatalf("missing timing values in payload: %q", body)
	}
}

func TestRequestTimingMiddlewareInsertsPayloadBeforeBodyEnd(t *testing.T) {
	handler := requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markRequestWorkComplete(r)
		_, _ = w.Write([]byte("<!doctype html><html><body>page</body></html>"))
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	body := response.Body.String()
	payloadPosition := strings.Index(body, "data-status-timings")
	bodyEndPosition := strings.Index(body, "</body>")
	if payloadPosition < 0 || bodyEndPosition < 0 || payloadPosition > bodyEndPosition {
		t.Fatalf("timing payload should be inserted before </body>: %q", body)
	}
}

func TestRequestTimingMiddlewareOmitsTimingWhenWorkWasNotMarkedComplete(t *testing.T) {
	handler := requestTimingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fragment"))
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(response.Body.String(), "data-status-timings") {
		t.Fatalf("unexpected timing payload for fragment response: %q", response.Body.String())
	}
}
