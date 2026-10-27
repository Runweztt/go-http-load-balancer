package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Create a temporary backend for testing.
func testBackend(name string) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}

			fmt.Fprint(w, name)
		}),
	)
}

// Verify that requests alternate between backends.
func TestRoundRobin(t *testing.T) {
	a := testBackend("A")
	c := testBackend("B")
	defer a.Close()
	defer c.Close()

	b, err := newBalancer([]string{a.URL, c.URL})
	if err != nil {
		t.Fatal(err)
	}

	b.checkHealth()

	for i, want := range []string{"A", "B", "A", "B"} {
		w := httptest.NewRecorder()

		b.ServeHTTP(
			w,
			httptest.NewRequest(http.MethodGet, "/", nil),
		)

		if got := w.Body.String(); got != want {
			t.Errorf(
				"request %d: got %q, want %q",
				i+1, got, want,
			)
		}
	}
}

// Verify routing when a backend fails.
func TestFailoverAndNoHealthyBackend(t *testing.T) {
	a := testBackend("A")
	c := testBackend("B")
	defer c.Close()

	b, err := newBalancer([]string{a.URL, c.URL})
	if err != nil {
		t.Fatal(err)
	}

	b.checkHealth()

	// Simulate backend A failing.
	a.Close()
	b.checkHealth()

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()

		b.ServeHTTP(
			w,
			httptest.NewRequest(http.MethodGet, "/", nil),
		)

		if w.Code != 200 || w.Body.String() != "B" {
			t.Fatalf(
				"failover: status=%d body=%q",
				w.Code, w.Body.String(),
			)
		}
	}

	// Simulate both backends being unavailable.
	c.Close()
	b.checkHealth()

	w := httptest.NewRecorder()

	b.ServeHTTP(
		w,
		httptest.NewRequest(http.MethodGet, "/", nil),
	)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// Verify routing under concurrent requests.
func TestConcurrentRequests(t *testing.T) {
	a := testBackend("A")
	c := testBackend("B")
	defer a.Close()
	defer c.Close()

	b, err := newBalancer([]string{a.URL, c.URL})
	if err != nil {
		t.Fatal(err)
	}

	b.checkHealth()

	const n = 100

	results := make(chan string, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()

			b.ServeHTTP(
				w,
				httptest.NewRequest(http.MethodGet, "/", nil),
			)

			results <- w.Body.String()
		}()
	}

	wg.Wait()
	close(results)

	counts := map[string]int{}

	for result := range results {
		counts[result]++
	}

	if counts["A"] != 50 || counts["B"] != 50 {
		t.Fatalf("unexpected distribution: %+v", counts)
	}
}
