package models

import "testing"

func strPtr(s string) *string { return &s }

func validBackend() Backend {
	return Backend{Host: "localhost", Port: 9001}
}

func baseSettings() Settings {
	return Settings{
		Port:       9000,
		Backends:   map[string]Backend{"b1": validBackend()},
		DBSettings: DBSettings{Filename: "db/x.db"},
	}
}

func TestValidateOK(t *testing.T) {
	if err := baseSettings().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateBadPort(t *testing.T) {
	s := baseSettings()
	s.Port = 0
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestValidateNoBackends(t *testing.T) {
	s := baseSettings()
	s.Backends = nil
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for no backends")
	}
}

func TestValidateBothDBSet(t *testing.T) {
	s := baseSettings()
	s.DBSettings = DBSettings{Filename: "db/x.db", Connstr: "postgres://x"}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when both filename and connstr set")
	}
}

func TestValidateNeitherDBSet(t *testing.T) {
	s := baseSettings()
	s.DBSettings = DBSettings{}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error when neither filename nor connstr set")
	}
}

func TestValidatePartialBasicAuth(t *testing.T) {
	s := baseSettings()
	b := validBackend()
	b.Username = strPtr("admin") // password missing
	s.Backends = map[string]Backend{"b1": b}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for username without password")
	}
}

func TestValidatePartialOAuth(t *testing.T) {
	s := baseSettings()
	b := validBackend()
	b.ClientId = strPtr("id") // secret + tokenURL missing
	s.Backends = map[string]Backend{"b1": b}
	if err := s.Validate(); err == nil {
		t.Fatal("expected error for partial oauth config")
	}
}
