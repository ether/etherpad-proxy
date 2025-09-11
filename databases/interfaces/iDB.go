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
}
