package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/arosenfeld2003/qwasar_eng_labs_events/internal/logger"
)

func main() {
	// Read RabbitMQ URL from env (even if we don't use it yet)
	rabbitURL := os.Getenv("RABBITMQ_URL")
	log.Printf("Starting marry-me service. RABBITMQ_URL=%q\n", rabbitURL)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "marry-me Go service is running 🚀")
	})

	return mux
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	log.Printf("Starting marry-me service. RABBITMQ_URL=%q\n", rabbitURL)

	port := "8080"
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}