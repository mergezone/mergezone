package provider

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	rateLimitLimitHeader     = "x-ratelimit-limit"
	rateLimitRemainingHeader = "x-ratelimit-remaining"
	rateLimitResetHeader     = "x-ratelimit-reset"
	rateLimitResourceHeader  = "x-ratelimit-resource"
	rateLimitUsedHeader      = "x-ratelimit-used"
	retryAfterHeader         = "retry-after"
)

// maxRateLimitErrorBodyBytes bounds how much of an error response body is read
// while looking for a secondary rate limit message.
const maxRateLimitErrorBodyBytes = 64 << 10

// LogRateLimitExceeded reports whether response indicates that a GitHub API
// rate limit has been exceeded and, if so, logs the rate limit state.
//
// A primary rate limit is a 403 or 429 response with no remaining requests. A
// secondary rate limit is a 403 or 429 response carrying a retry-after header
// or a message that mentions a secondary rate limit.
//
// See https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#exceeding-the-rate-limit
func LogRateLimitExceeded(logger *slog.Logger, response *http.Response) bool {
	if response == nil {
		return false
	}
	if logger == nil {
		logger = slog.Default()
	}

	if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusTooManyRequests {
		return false
	}

	remaining := response.Header.Get(rateLimitRemainingHeader)
	retryAfter := response.Header.Get(retryAfterHeader)
	message := rateLimitErrorMessage(response)
	secondary := retryAfter != "" || strings.Contains(strings.ToLower(message), "secondary rate limit")

	if remaining != "0" && !secondary {
		return false
	}

	attributes := []any{
		"status", response.StatusCode,
		"status_text", response.Status,
		"remaining", remaining,
		"limit", response.Header.Get(rateLimitLimitHeader),
		"used", response.Header.Get(rateLimitUsedHeader),
		"resource", response.Header.Get(rateLimitResourceHeader),
	}
	if response.Request != nil {
		attributes = append(attributes,
			"method", response.Request.Method,
			"url", response.Request.URL.String(),
		)
	}
	if retryAfter != "" {
		attributes = append(attributes, "retry_after_seconds", retryAfter)
	}
	if reset := response.Header.Get(rateLimitResetHeader); reset != "" {
		if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil {
			resetAt := time.Unix(epoch, 0).UTC()
			attributes = append(attributes,
				"reset", resetAt.Format(time.RFC3339),
				"reset_in", time.Until(resetAt).Round(time.Second).String(),
			)
		} else {
			attributes = append(attributes, "reset", reset)
		}
	}
	if message != "" {
		attributes = append(attributes, "message", message)
	}

	if secondary {
		logger.Warn("GitHub secondary rate limit exceeded", attributes...)
	} else {
		logger.Warn("GitHub primary rate limit exceeded", attributes...)
	}

	return true
}

type rateLimitErrorResponse struct {
	Message string `json:"message"`
}

// rateLimitErrorMessage extracts the "message" field from an error response
// body without consuming it, so callers can still read the body afterwards.
func rateLimitErrorMessage(response *http.Response) string {
	if response.Body == nil {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxRateLimitErrorBodyBytes))
	response.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), response.Body))
	if err != nil {
		return ""
	}

	var errorResponse rateLimitErrorResponse
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return ""
	}
	return errorResponse.Message
}
