package models

import (
	"sync"
	"testing"
)

func TestBackendStateSnapshotIsolated(t *testing.T) {
	var s BackendState
	s.SetState([]string{"a", "b"}, []string{"a"})
	avail := s.SnapshotAvailable()
	avail[0] = "mutated"
	if got := s.SnapshotAvailable(); got[0] != "a" {
		t.Fatalf("snapshot should be a copy, got %v", got)
	}
}

func TestBackendStateIsUp(t *testing.T) {
	var s BackendState
	s.SetState(nil, []string{"a"})
	if !s.IsUp("a") {
		t.Fatal("a should be up")
	}
	if s.IsUp("b") {
		t.Fatal("b should not be up")
	}
}

func TestBackendStateConcurrent(t *testing.T) {
	var s BackendState
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); s.SetState([]string{"a"}, []string{"a"}) }()
		go func() { defer wg.Done(); _ = s.SnapshotUp() }()
	}
	wg.Wait()
}
