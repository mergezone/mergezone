package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// countingTransport is a stub RoundTripper that records how many requests reach
// the network and always returns the configured response.
type countingTransport struct {
	calls    int
	status   int
	body     string
	response func(*http.Request) *http.Response
}

func (t *countingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.calls++
	if t.response != nil {
		return t.response(request), nil
	}
	return newTestResponse(t.status, t.body), nil
}

func newTestResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	return recorder
}

func spanAttributes(span sdktrace.ReadOnlySpan) map[string]any {
	attributes := make(map[string]any, len(span.Attributes()))
	for _, attribute := range span.Attributes() {
		attributes[string(attribute.Key)] = attribute.Value.AsInterface()
	}
	return attributes
}

func hasEvent(span sdktrace.ReadOnlySpan, name string) bool {
	for _, event := range span.Events() {
		if event.Name == name {
			return true
		}
	}
	return false
}

func doRequest(t *testing.T, transport http.RoundTripper, method, url, accept string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

func TestCachingTransportCachesRepeatedGET(t *testing.T) {
	base := &countingTransport{status: http.StatusOK, body: `{"ok":true}`}
	transport := newCachingTransport(base)

	for i := range 2 {
		response := doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json")
		if body := readBody(t, response); body != `{"ok":true}` {
			t.Fatalf("request %d body = %q, want %q", i, body, `{"ok":true}`)
		}
	}

	if base.calls != 1 {
		t.Fatalf("network calls = %d, want 1", base.calls)
	}
}

func TestCachingTransportSeparatesResponsesByAccept(t *testing.T) {
	base := &countingTransport{status: http.StatusOK, body: "payload"}
	transport := newCachingTransport(base)
	const url = "https://api.github.com/repos/o/r/pulls/1"

	doRequest(t, transport, http.MethodGet, url, "application/vnd.github+json").Body.Close()
	doRequest(t, transport, http.MethodGet, url, "application/vnd.github.v3.diff").Body.Close()

	if base.calls != 2 {
		t.Fatalf("network calls = %d, want 2 (JSON and diff are distinct resources)", base.calls)
	}
}

func TestCachingTransportDoesNotCacheNonOKResponses(t *testing.T) {
	base := &countingTransport{status: http.StatusInternalServerError, body: "boom"}
	transport := newCachingTransport(base)

	for range 2 {
		response := doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json")
		if response.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusInternalServerError)
		}
		response.Body.Close()
	}

	if base.calls != 2 {
		t.Fatalf("network calls = %d, want 2", base.calls)
	}
}

func TestCachingTransportIgnoresNonGETRequests(t *testing.T) {
	base := &countingTransport{status: http.StatusOK, body: "payload"}
	transport := newCachingTransport(base)

	for range 2 {
		response := doRequest(t, transport, http.MethodPost, "https://api.github.com/repos/o/r/pulls", "application/vnd.github+json")
		response.Body.Close()
	}

	if base.calls != 2 {
		t.Fatalf("network calls = %d, want 2", base.calls)
	}
}

func TestCachingTransportRecordsHitAndMissTelemetry(t *testing.T) {
	recorder := installSpanRecorder(t)
	base := &countingTransport{status: http.StatusOK, body: "payload"}
	transport := newCachingTransport(base)

	doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json").Body.Close()
	doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json").Body.Close()

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended spans = %d, want 2", len(spans))
	}

	miss := spanAttributes(spans[0])
	if miss["cache.hit"] != false || miss["cache.result"] != "miss" {
		t.Fatalf("first span attributes = %#v, want cache miss", miss)
	}
	if !hasEvent(spans[0], "cache.miss") {
		t.Fatalf("first span events = %v, want cache.miss", spans[0].Events())
	}

	hit := spanAttributes(spans[1])
	if hit["cache.hit"] != true || hit["cache.result"] != "hit" {
		t.Fatalf("second span attributes = %#v, want cache hit", hit)
	}
	if !hasEvent(spans[1], "cache.hit") {
		t.Fatalf("second span events = %v, want cache.hit", spans[1].Events())
	}
}

func TestCachingTransportReturnsFreshBodyPerHit(t *testing.T) {
	base := &countingTransport{status: http.StatusOK, body: "payload"}
	transport := newCachingTransport(base)

	first := doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json")
	if body := readBody(t, first); body != "payload" {
		t.Fatalf("first body = %q, want %q", body, "payload")
	}
	second := doRequest(t, transport, http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", "application/vnd.github+json")
	if body := readBody(t, second); body != "payload" {
		t.Fatalf("second body = %q, want %q", body, "payload")
	}
}

func TestCachingTransportPreservesHeadersForPagination(t *testing.T) {
	base := &countingTransport{
		response: func(*http.Request) *http.Response {
			response := newTestResponse(http.StatusOK, "payload")
			response.Header.Set("Link", `<https://api.github.com/repos/o/r/pulls?page=2>; rel="next"`)
			return response
		},
	}
	transport := newCachingTransport(base)
	const url = "https://api.github.com/repos/o/r/pulls?page=1"

	doRequest(t, transport, http.MethodGet, url, "application/vnd.github+json").Body.Close()
	response := doRequest(t, transport, http.MethodGet, url, "application/vnd.github+json")
	defer response.Body.Close()

	if !responseHasNextPage(response) {
		t.Fatalf("cached response lost Link header: %v", response.Header)
	}
}

func TestCachingTransportPassesContextToNetwork(t *testing.T) {
	base := &countingTransport{
		response: func(request *http.Request) *http.Response {
			if request.Context() == nil {
				t.Fatal("request context is nil")
			}
			return newTestResponse(http.StatusOK, "payload")
		},
	}
	transport := newCachingTransport(base)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.github.com/repos/o/r/pulls?page=1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatalf("round trip: %v", err)
	}
}
