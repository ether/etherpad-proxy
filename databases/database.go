package databases

import (
	"fmt"

	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/databases/postgres"
	"github.com/ether/etherpad-proxy/databases/sqlite"
	"github.com/ether/etherpad-proxy/models"
)

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

func CreateNewDatabase(settings models.Settings) (interfaces.IDB, error) {
	var dbType = DBTypePostgres
	if settings.DBSettings.Filename != "" {
		dbType = DBTypeSQLite
	}

	if settings.DBSettings.Connstr != "" {
		dbType = DBTypePostgres
	}

	switch dbType {
	case DBTypePostgres:
		return postgres.NewPostgresDB(settings.DBSettings.Connstr)
	case DBTypeSQLite:
		return sqlite.NewSQLiteDB(settings.DBSettings.Filename)
	}
	return nil, fmt.Errorf("unknown database type: %s", dbType)
}
