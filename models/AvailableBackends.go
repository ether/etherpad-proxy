package models

import (
	"slices"
	"sync"
)

// BackendState holds the per-instance view of which backends are up and which
// still have capacity (available). It is safe for concurrent use.
type BackendState struct {
	mu        sync.RWMutex
	up        []string
	available []string
}

// SetState atomically replaces both lists with copies of the provided slices.
func (b *BackendState) SetState(available, up []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.available = append([]string(nil), available...)
	b.up = append([]string(nil), up...)
}

// SnapshotAvailable returns a copy of the available list.
func (b *BackendState) SnapshotAvailable() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.available...)
}

// SnapshotUp returns a copy of the up list.
func (b *BackendState) SnapshotUp() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.up...)
}

// IsUp reports whether backend is currently up.
func (b *BackendState) IsUp(backend string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return slices.Contains(b.up, backend)
}
