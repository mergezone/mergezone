package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"merge/internal/handler"
	"merge/internal/provider/github"
	"merge/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var commit = "unknown"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, logger)
	if err != nil {
		logger.Error("configure OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Error("shutdown OpenTelemetry", "error", err)
		}
	}()

	s := handler.Server{
		BaseURL: "https://www.merge.zone",
		Commit:  commit,
		Port:    8080,
		Router:  mux.NewRouter(),
		Logger:  logger,
		Provider: github.GitHub{
			Token:  os.Getenv("PROVIDER_TOKEN_GITHUB"),
			Logger: logger,
		},
	}

	s.Router.Use(otelhttp.NewMiddleware("merge/http"))
	if err := s.ListenAndServe(ctx); err != nil {
		s.Logger.Error(err.Error())
	}
}
