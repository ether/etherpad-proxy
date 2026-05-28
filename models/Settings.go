package models

import (
	"errors"
	"fmt"
)

type Settings struct {
	Port               int                `json:"port"`
	ManagementPort     int                `json:"managementPort"`
	Backends           map[string]Backend `json:"backends"`
	MaxPadsPerInstance int                `json:"maxPadsPerInstance"`
	CheckInterval      int64              `json:"checkInterval"`
	DBSettings         DBSettings         `json:"dbSettings"`
}

type DBSettings struct {
	Filename string `json:"filename"`
	Connstr  string `json:"postgresConnstr"`
}

type Backend struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	ClientId     *string  `json:"clientId"`
	ClientSecret *string  `json:"clientSecret"`
	Scopes       []string `json:"scopes"`
	TokenURL     *string  `json:"tokenUrl"`
	Username     *string  `json:"username"`
	Password     *string  `json:"password"`
}

func validPort(p int) bool { return p > 0 && p <= 65535 }

// Validate checks that the settings are internally consistent and usable.
func (s Settings) Validate() error {
	if !validPort(s.Port) {
		return fmt.Errorf("port must be between 1 and 65535, got %d", s.Port)
	}
	if s.ManagementPort != 0 && !validPort(s.ManagementPort) {
		return fmt.Errorf("managementPort must be between 1 and 65535, got %d", s.ManagementPort)
	}
	if len(s.Backends) == 0 {
		return errors.New("at least one backend must be configured")
	}
	for name, b := range s.Backends {
		if b.Host == "" {
			return fmt.Errorf("backend %q is missing host", name)
		}
		if !validPort(b.Port) {
			return fmt.Errorf("backend %q has invalid port %d", name, b.Port)
		}
		if (b.Username != nil) != (b.Password != nil) {
			return fmt.Errorf("backend %q must set both username and password", name)
		}
		hasOAuth := b.ClientId != nil && b.ClientSecret != nil && b.TokenURL != nil
		if (b.ClientId != nil || b.ClientSecret != nil || b.TokenURL != nil) && !hasOAuth {
			return fmt.Errorf("backend %q must set clientId, clientSecret and tokenUrl together", name)
		}
	}
	hasFile := s.DBSettings.Filename != ""
	hasConn := s.DBSettings.Connstr != ""
	if hasFile == hasConn {
		return errors.New("exactly one of dbSettings.filename or dbSettings.postgresConnstr must be set")
	}
	return nil
}
