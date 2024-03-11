package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
)

func getRoot(w http.ResponseWriter, r *http.Request) {
	slog.Info("got / request")
	w.WriteHeader(200)
	w.Write([]byte("Hello from pets"))
}

func main() {
	http.HandleFunc("/", getRoot)
	slog.Info("Running on port :3000...")

	err := http.ListenAndServe(":3000", nil)
	if errors.Is(err, http.ErrServerClosed) {
		slog.Error("server closed: w", err)
	} else if err != nil {
		slog.Error("error starting server: %w", err)
		os.Exit(1)
	}
}
