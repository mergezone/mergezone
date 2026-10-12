package provider

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func testResponse(status int, body string, headers map[string]string) *http.Response {
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/repos/octocat/hello-world/pulls", nil)
	response := &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{},
		Body:       http.NoBody,
		Request:    request,
	}
	if body != "" {
		response.Body = io.NopCloser(strings.NewReader(body))
	}
	for key, value := range headers {
		response.Header.Set(key, value)
	}
	return response
}

func TestLogRateLimitExceeded(t *testing.T) {
	tests := []struct {
		name      string
		response  *http.Response
		wantLog   bool
		wantLevel string
		wantText  string
	}{
		{
			name:      "primary rate limit with no remaining requests",
			response:  testResponse(http.StatusForbidden, "", map[string]string{"x-ratelimit-remaining": "0"}),
			wantLog:   true,
			wantLevel: "WARN",
			wantText:  "primary rate limit exceeded",
		},
		{
			name:      "primary rate limit via 429",
			response:  testResponse(http.StatusTooManyRequests, "", map[string]string{"x-ratelimit-remaining": "0"}),
			wantLog:   true,
			wantLevel: "WARN",
			wantText:  "primary rate limit exceeded",
		},
		{
			name:      "secondary rate limit via retry-after header",
			response:  testResponse(http.StatusForbidden, "", map[string]string{"retry-after": "60"}),
			wantLog:   true,
			wantLevel: "WARN",
			wantText:  "secondary rate limit exceeded",
		},
		{
			name:      "secondary rate limit via error message",
			response:  testResponse(http.StatusTooManyRequests, `{"message":"You have exceeded a secondary rate limit."}`, nil),
			wantLog:   true,
			wantLevel: "WARN",
			wantText:  "secondary rate limit exceeded",
		},
		{
			name:      "primary rate limit retains response body for caller",
			response:  testResponse(http.StatusForbidden, `{"message":"API rate limit exceeded"}`, map[string]string{"x-ratelimit-remaining": "0"}),
			wantLog:   true,
			wantLevel: "WARN",
			wantText:  "primary rate limit exceeded",
		},
		{
			name:     "forbidden without rate limit signals",
			response: testResponse(http.StatusForbidden, `{"message":"Bad credentials"}`, map[string]string{"x-ratelimit-remaining": "42"}),
			wantLog:  false,
		},
		{
			name:     "successful response",
			response: testResponse(http.StatusOK, "", nil),
			wantLog:  false,
		},
		{
			name:     "nil response",
			response: nil,
			wantLog:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buffer, nil))

			if got := LogRateLimitExceeded(logger, tt.response); got != tt.wantLog {
				t.Fatalf("LogRateLimitExceeded() = %t, want %t", got, tt.wantLog)
			}

			logged := buffer.String()
			if !tt.wantLog {
				if logged != "" {
					t.Errorf("expected no log output, got %q", logged)
				}
				return
			}
			if !strings.Contains(logged, tt.wantLevel) {
				t.Errorf("expected level %q in log output, got %q", tt.wantLevel, logged)
			}
			if !strings.Contains(logged, tt.wantText) {
				t.Errorf("expected %q in log output, got %q", tt.wantText, logged)
			}
		})
	}
}

func TestLogRateLimitExceeded_PreservesResponseBody(t *testing.T) {
	const body = `{"message":"API rate limit exceeded for 1.2.3.4."}`
	response := testResponse(http.StatusForbidden, body, map[string]string{"x-ratelimit-remaining": "0"})

	LogRateLimitExceeded(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), response)

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		t.Fatalf("reading body after rate limit check: %v", err)
	}
	if buffer.String() != body {
		t.Errorf("body = %q, want %q", buffer.String(), body)
	}
}

func TestLogRateLimitExceeded_NilLoggerDoesNotPanic(t *testing.T) {
	response := testResponse(http.StatusForbidden, "", map[string]string{"x-ratelimit-remaining": "0"})
	if !LogRateLimitExceeded(nil, response) {
		t.Error("expected rate limit to be detected with nil logger")
	}
}
