package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"merge/internal/provider"
	"merge/internal/telemetry"
)

const responseTailSize = 256

type requestStartKey struct{}
type requestSpanKey struct{}

type requestTiming struct {
	startedAt      time.Time
	gatherDuration time.Duration
	logicDuration  time.Duration
	workEnded      time.Time
}

// requestTimingMiddleware streams page responses while retaining only a short
// tail, so it can append accurate timing data before the document's closing body tag.
func requestTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.Tracer("merge/handler").Start(r.Context(), "http.page",
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("url.path", r.URL.Path),
			),
		)
		defer span.End()
		ctx = context.WithValue(ctx, requestSpanKey{}, span)
		r = r.WithContext(ctx)

		timing := &requestTiming{startedAt: time.Now()}
		response := &timingResponseWriter{dst: w}
		ctx = context.WithValue(ctx, requestStartKey{}, timing)
		next.ServeHTTP(response, r.WithContext(ctx))

		var payload []byte
		if !timing.workEnded.IsZero() {
			finishedAt := time.Now()
			logicDuration := timing.logicDuration
			templateDuration := finishedAt.Sub(timing.workEnded)
			payload = []byte(fmt.Sprintf(
				`<div hidden data-status-timings data-gather-ms="%.3f" data-logic-ms="%.3f" data-template-ms="%.3f"></div>`,
				float64(timing.gatherDuration)/float64(time.Millisecond),
				float64(logicDuration)/float64(time.Millisecond),
				float64(templateDuration)/float64(time.Millisecond),
			))
		}
		response.finish(payload, r.Header.Get("HX-Request") != "true")
		span.SetAttributes(attribute.Int("http.response.status_code", response.statusCode()))
		if response.statusCode() >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(response.statusCode()))
		}
	})
}

// recordProviderTimings records measured GitHub wait time. Logic is calculated
// from total request work minus this duration when rendering begins.
func recordProviderTimings(r *http.Request, providerTimings provider.PullRequestTimings) {
	if timing, ok := r.Context().Value(requestStartKey{}).(*requestTiming); ok {
		timing.gatherDuration += providerTimings.Gather
	}
}

// markRequestWorkComplete ends application logic and starts template timing.
func markRequestWorkComplete(r *http.Request) {
	if timing, ok := r.Context().Value(requestStartKey{}).(*requestTiming); ok {
		now := time.Now()
		timing.logicDuration = now.Sub(timing.startedAt) - timing.gatherDuration
		if timing.logicDuration < 0 {
			timing.logicDuration = 0
		}
		timing.workEnded = now
	}
}

func recordRequestError(r *http.Request, err error) {
	if span, ok := r.Context().Value(requestSpanKey{}).(trace.Span); ok {
		telemetry.RecordError(span, err)
	}
}

type timingResponseWriter struct {
	dst         http.ResponseWriter
	tail        []byte
	status      int
	wroteHeader bool
}

func (w *timingResponseWriter) Header() http.Header {
	return w.dst.Header()
}

func (w *timingResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *timingResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.dst.WriteHeader(status)
}

func (w *timingResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	combined := make([]byte, 0, len(w.tail)+len(data))
	combined = append(combined, w.tail...)
	combined = append(combined, data...)
	if len(combined) <= responseTailSize {
		w.tail = combined
		return len(data), nil
	}

	flushLength := len(combined) - responseTailSize
	if _, err := w.dst.Write(combined[:flushLength]); err != nil {
		return 0, err
	}
	w.tail = append(w.tail[:0], combined[flushLength:]...)
	return len(data), nil
}

func (w *timingResponseWriter) finish(payload []byte, fullDocument bool) {
	if len(payload) > 0 && fullDocument {
		if bodyEnd := bytes.LastIndex(w.tail, []byte("</body>")); bodyEnd >= 0 {
			inserted := make([]byte, 0, len(w.tail)+len(payload))
			inserted = append(inserted, w.tail[:bodyEnd]...)
			inserted = append(inserted, payload...)
			inserted = append(inserted, w.tail[bodyEnd:]...)
			w.tail = inserted
		} else {
			w.tail = append(w.tail, payload...)
		}
	} else if len(payload) > 0 {
		w.tail = append(w.tail, payload...)
	}

	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.dst.Write(w.tail)
}
