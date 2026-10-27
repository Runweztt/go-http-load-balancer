package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// backend represents one server receiving traffic.
type backend struct {
	url     *url.URL
	proxy   *httputil.ReverseProxy
	healthy bool
}

// balancer manages the available backend servers.
type balancer struct {
	mu       sync.RWMutex
	backends []*backend
	next     atomic.Uint64
	client   *http.Client
}

// newBalancer constructs our load balancer.
func newBalancer(addresses []string) (*balancer, error) {
	b := &balancer{
		client: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}

	for _, address := range addresses {
		u, err := url.Parse(address)

		if err != nil || u.Scheme != "http" || u.Host == "" {
			return nil, &url.Error{
				Op:  "parse backend",
				URL: address,
				Err: errInvalidBackend,
			}
		}

		// Create one reusable proxy per backend.
		proxy := httputil.NewSingleHostReverseProxy(u)

		proxy.ErrorHandler = func(
			w http.ResponseWriter,
			r *http.Request,
			err error,
		) {
			log.Printf("proxy request failed: %v", err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}

		b.backends = append(b.backends, &backend{
			url:   u,
			proxy: proxy,
		})
	}

	if len(b.backends) == 0 {
		return nil, errNoBackends
	}

	return b, nil
}

// checkHealth tests each backend's /health endpoint.
func (b *balancer) checkHealth() {
	for _, server := range b.backends {
		req, err := http.NewRequest(
			http.MethodGet,
			server.url.String()+"/health",
			nil,
		)

		healthy := false

		if err == nil {
			response, err := b.client.Do(req)

			if err == nil {
				healthy = response.StatusCode == http.StatusOK
				response.Body.Close()
			}
		}

		// Protect the shared healthy flag.
		b.mu.Lock()

		changed := server.healthy != healthy
		server.healthy = healthy

		b.mu.Unlock()

		if changed {
			log.Printf(
				"backend %s healthy=%t",
				server.url,
				healthy,
			)
		}
	}
}

// startChecks repeats the health check periodically.
func (b *balancer) startChecks(
	stop <-chan struct{},
	interval time.Duration,
) {
	b.checkHealth()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.checkHealth()

		case <-stop:
			return
		}
	}
}

// ServeHTTP handles incoming client requests.
func (b *balancer) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	b.mu.RLock()

	available := make([]*backend, 0, len(b.backends))

	// Only select healthy backend servers.
	for _, server := range b.backends {
		if server.healthy {
			available = append(available, server)
		}
	}

	b.mu.RUnlock()

	if len(available) == 0 {
		http.Error(
			w,
			"no healthy backends",
			http.StatusServiceUnavailable,
		)
		return
	}

	// Concurrency-safe round-robin selection.
	index := (b.next.Add(1) - 1) % uint64(len(available))
	selected := available[index]

	log.Printf(
		"routing %s %s to %s",
		r.Method,
		r.URL.Path,
		selected.url,
	)

	// Forward the incoming HTTP request.
	selected.proxy.ServeHTTP(w, r)
}
