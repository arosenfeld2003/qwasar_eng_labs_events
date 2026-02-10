package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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
	log.Fatal(http.ListenAndServe(":"+port, newMux()))
}
