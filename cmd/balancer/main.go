package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	errInvalidBackend = errors.New(
		"backend must be an http URL with a host",
	)

	errNoBackends = errors.New(
		"at least one backend is required",
	)
)

func main() {
	// Read backend addresses from the environment.
	addresses := os.Getenv("BACKENDS")

	if addresses == "" {
		addresses = "http://localhost:8081,http://localhost:8082"
	}

	// Read the port.
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	// Create the load balancer.
	b, err := newBalancer(strings.Split(addresses, ","))

	if err != nil {
		log.Fatal(err)
	}

	// Run health checks concurrently.
	go b.startChecks(make(chan struct{}), time.Second)

	log.Printf("load balancer listening on port %s", port)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           b,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
