package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

// Response represents the JSON returned by a backend.
type Response struct {
	Server string `json:"server"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

func main() {
	name := flag.String("name", "Backend A", "backend name")
	port := flag.String("port", "8081", "listening port")
	flag.Parse()

	mux := http.NewServeMux()

	// Health endpoint: the load balancer will call this.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Normal application endpoint.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s received %s %s", *name, r.Method, r.URL.Path)

		w.Header().Set("Content-Type", "application/json")

		response := Response{
			Server: *name,
			Method: r.Method,
			Path:   r.URL.Path,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("encoding response: %v", err)
		}
	})

	log.Printf("%s listening on port %s", *name, *port)
	log.Fatal(http.ListenAndServe(":"+*port, mux))
}
