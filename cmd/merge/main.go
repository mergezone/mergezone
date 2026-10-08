package main

import (
	"log/slog"
	"os"

	"github.com/gorilla/mux"

	"merge/internal/handler"
	"merge/internal/provider/github"
)

func main() {
	s := handler.Server{
		BaseURL: "https://www.merge.zone",
		Port:    8080,
		Router:  mux.NewRouter(),
		Logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Provider: github.GitHub{
			Token: os.Getenv("PROVIDER_TOKEN_GITHUB"),
		},
	}

	if err := s.ListenAndServe(); err != nil {
		s.Logger.Error(err.Error())
	}
}
