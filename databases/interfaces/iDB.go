package interfaces

import "github.com/ether/etherpad-proxy/models"

type IDB interface {
	Close() error
	Get(id string) (*models.DBBackend, error)
	CleanUpPads(padIds []string, padPrefix string) error
	RecordClash(id string, data string) error
	Set(id string, backend models.DBBackend) error
	GetAllPads() (map[string]string, error)
	GetClashByPadID(id string) ([]string, error)
	// Assign stores candidate as the backend for padId only if no backend is
	// already stored, and returns the backend now authoritative for padId (the
	// pre-existing one if there was a race, otherwise candidate).
	Assign(padId string, candidate string) (string, error)
}
