package github

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"merge/internal/telemetry"
)

// responseCacheDuration bounds how long a GitHub response may be reused. The
// cache is keyed by the request that produced it, so it sits below the
// provider's filtering and stamping work.
const responseCacheDuration = 5 * time.Minute

// cacheSystemName labels the cache backend in telemetry.
const cacheSystemName = "go-cache"

// cachingTransport is an http.RoundTripper that memoizes successful GET
// responses for responseCacheDuration. It wraps otelhttp rather than sitting
// inside it: a cache hit returns before the wrapped transport runs, so a hit
// produces only a github.cache span while a miss produces a github.cache span
// followed by the usual HTTP client span.
type cachingTransport struct {
	base  http.RoundTripper
	cache *cache.Cache
}

func newCachingTransport(base http.RoundTripper) *cachingTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &cachingTransport{
		base:  base,
		cache: cache.New(responseCacheDuration, 10*time.Minute),
	}
}

type cachedResponse struct {
	statusCode int
	header     http.Header
	body       []byte
}

// cacheKey identifies the upstream resource. Accept is included because GitHub
// serves both JSON and diffs from the same pull request URL, distinguished only
// by content negotiation.
func cacheKey(request *http.Request) string {
	return request.Method + " " + request.URL.String() + " " + request.Header.Get("Accept")
}

func (t *cachingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodGet {
		return t.base.RoundTrip(request)
	}

	ctx, span := telemetry.Tracer("merge/github").Start(request.Context(), "github.cache",
		trace.WithAttributes(
			attribute.String("cache.system", cacheSystemName),
			attribute.String("http.request.method", request.Method),
			attribute.String("url.full", request.URL.String()),
		),
	)
	defer span.End()

	key := cacheKey(request)
	if value, found := t.cache.Get(key); found {
		cached := value.(cachedResponse)
		markCacheHit(span)
		span.SetAttributes(attribute.Int("http.response.status_code", cached.statusCode))
		return cached.response(request), nil
	}

	markCacheMiss(span)
	response, err := t.base.RoundTrip(request.WithContext(ctx))
	if err != nil {
		telemetry.RecordError(span, err)
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	if response.StatusCode != http.StatusOK {
		return response, nil
	}

	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		telemetry.RecordError(span, err)
		return nil, err
	}
	t.cache.Set(key, cachedResponse{
		statusCode: response.StatusCode,
		header:     response.Header.Clone(),
		body:       body,
	}, cache.DefaultExpiration)
	span.SetAttributes(attribute.Int("cache.entries", t.cache.ItemCount()))

	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}

func (c cachedResponse) response(request *http.Request) *http.Response {
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", c.statusCode, http.StatusText(c.statusCode)),
		StatusCode:    c.statusCode,
		Header:        c.header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(c.body)),
		ContentLength: int64(len(c.body)),
		Request:       request,
	}
}

func markCacheHit(span trace.Span) {
	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.String("cache.result", "hit"),
	)
	span.AddEvent("cache.hit")
}

func markCacheMiss(span trace.Span) {
	span.SetAttributes(
		attribute.Bool("cache.hit", false),
		attribute.String("cache.result", "miss"),
	)
	span.AddEvent("cache.miss")
}
