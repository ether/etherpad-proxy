package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httputil"
	"testing"

	"github.com/ether/etherpad-proxy/models"
	"go.uber.org/zap"
)

type fakeDB struct {
	pads        map[string]string
	clashes     map[string][]string
	assignCalls int
}

func newFakeDB() *fakeDB {
	return &fakeDB{pads: map[string]string{}, clashes: map[string][]string{}}
}

func (f *fakeDB) Close() error { return nil }
func (f *fakeDB) Get(id string) (*models.DBBackend, error) {
	if b, ok := f.pads[id]; ok {
		return &models.DBBackend{Backend: b}, nil
	}
	return nil, sql.ErrNoRows
}
func (f *fakeDB) CleanUpPads(_ []string, _ string) error { return nil }
func (f *fakeDB) RecordClash(id string, data string) error {
	f.clashes[id] = append(f.clashes[id], data)
	return nil
}
func (f *fakeDB) Set(id string, backend models.DBBackend) error {
	f.pads[id] = backend.Backend
	return nil
}
func (f *fakeDB) GetAllPads() (map[string]string, error) { return f.pads, nil }
func (f *fakeDB) GetClashByPadID(id string) ([]string, error) {
	return f.clashes[id], nil
}
func (f *fakeDB) Assign(id string, candidate string) (string, error) {
	f.assignCalls++
	if b, ok := f.pads[id]; ok {
		return b, nil
	}
	f.pads[id] = candidate
	return candidate, nil
}

func newTestHandler(db *fakeDB, available, up []string) *ProxyHandler {
	state := &models.BackendState{}
	state.SetState(available, up)
	return &ProxyHandler{
		p:      map[string]httputil.ReverseProxy{},
		logger: zap.NewNop().Sugar(),
		db:     db,
		state:  state,
		static: newStaticResources(),
	}
}

func mustReq(path string) *http.Request {
	r, _ := http.NewRequest("GET", path, nil)
	return r
}

func TestChooseBackendNoBackends(t *testing.T) {
	h := newTestHandler(newFakeDB(), nil, nil)
	pad := "pad1"
	if _, err := h.chooseBackend(&pad, mustReq("/p/pad1")); err == nil {
		t.Fatal("expected error when no backends available")
	}
}

func TestChooseBackendAssignsNewPad(t *testing.T) {
	db := newFakeDB()
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b1" {
		t.Fatalf("expected b1, got %s", got)
	}
	if db.assignCalls != 1 {
		t.Fatalf("expected Assign called once, got %d", db.assignCalls)
	}
}

func TestChooseBackendUsesStoredWhenUp(t *testing.T) {
	db := newFakeDB()
	db.pads["pad1"] = "b2"
	h := newTestHandler(db, []string{"b1"}, []string{"b1", "b2"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b2" {
		t.Fatalf("expected stored backend b2, got %s", got)
	}
}

func TestChooseBackendReassignsWhenStoredDown(t *testing.T) {
	db := newFakeDB()
	db.pads["pad1"] = "down"
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	got, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	if err != nil {
		t.Fatalf("chooseBackend: %v", err)
	}
	if got != "b1" {
		t.Fatalf("expected reassign to b1, got %s", got)
	}
	if db.pads["pad1"] != "b1" {
		t.Fatalf("expected DB updated to b1, got %s", db.pads["pad1"])
	}
}

func TestChooseBackendClash(t *testing.T) {
	db := newFakeDB()
	db.clashes["pad1"] = []string{"b1", "b2"}
	h := newTestHandler(db, []string{"b1"}, []string{"b1"})
	pad := "pad1"
	_, err := h.chooseBackend(&pad, mustReq("/p/pad1"))
	var clash *ClashInPadId
	if !errors.As(err, &clash) {
		t.Fatalf("expected ClashInPadId, got %v", err)
	}
}
