package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"testing"

	"github.com/ether/etherpad-proxy/models"
)

// newStatsBackend starts a test server replying to /stats with the given
// activePads count, and returns its host and port.
func newStatsBackend(t *testing.T, activePads int) (string, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stats" {
			fmt.Fprintf(w, `{"activePads": %d}`, activePads)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	return u.Hostname(), port
}

func TestCheckAvailabilityCapacityAndDown(t *testing.T) {
	okHost, okPort := newStatsBackend(t, 1)       // under capacity -> up + available
	fullHost, fullPort := newStatsBackend(t, 99)  // over capacity -> up but not available

	settings := models.Settings{
		MaxPadsPerInstance: 5,
		Backends: map[string]models.Backend{
			"ok":   {Host: okHost, Port: okPort},
			"full": {Host: fullHost, Port: fullPort},
			"down": {Host: "127.0.0.1", Port: 1}, // nothing listening -> not up
		},
	}

	got := checkAvailability(settings)

	if !slices.Contains(got.Up, "ok") || !slices.Contains(got.Up, "full") {
		t.Fatalf("expected ok and full to be up, got %v", got.Up)
	}
	if slices.Contains(got.Up, "down") {
		t.Fatalf("down backend should not be up, got %v", got.Up)
	}
	if !slices.Contains(got.Available, "ok") {
		t.Fatalf("ok should be available, got %v", got.Available)
	}
	if slices.Contains(got.Available, "full") {
		t.Fatalf("full backend should not be available, got %v", got.Available)
	}
}
