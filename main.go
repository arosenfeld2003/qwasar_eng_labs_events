package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/logger"
)

func main() {
	verbose := flag.Bool("verbose", false, "pretty logs (development)")
	logLevel := flag.String("log-level", "info", "debug|info|warn|error")
	flag.Parse()

	log := logger.Setup(logger.Config{
		Verbose:   *verbose,
		Level:     *logLevel,
		Component: "marry-me",
		Out:       os.Stdout,
	})

	rabbitURL := os.Getenv("RABBITMQ_URL")
	log.Info().Str("rabbitmq_url", rabbitURL).Msg("Starting marry-me service")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "marry-me Go service is running 🚀")
	})

	port := "8080"
	log.Info().Str("port", port).Msg("Listening")
	log.Fatal().Err(http.ListenAndServe(":"+port, nil)).Msg("server stopped")
}