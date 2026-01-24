package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Read RabbitMQ URL from env (even if we don't use it yet)
	rabbitURL := os.Getenv("RABBITMQ_URL")
	log.Printf("Starting marry-me service. RABBITMQ_URL=%q\n", rabbitURL)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "marry-me Go service is running 🚀")
	})

	port := "8080"
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
