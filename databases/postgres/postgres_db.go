package postgres

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/models"
)

type Postgres struct {
	Conn *sql.DB
}

var _ interfaces.IDB = (*Postgres)(nil)

func NewPostgresDB(connstr string) (*Postgres, error) {
	conn, err := sql.Open("postgres", connstr)
	if err != nil {
		return nil, err
	}

	db := &Postgres{
		Conn: conn,
	}

	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT, backend TEXT, PRIMARY KEY (id))"); err != nil {
		return nil, err
	}

	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS clashes (id TEXT, data TEXT, PRIMARY KEY (id, data))"); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *Postgres) Close() error {
	return db.Conn.Close()
}

func (db *Postgres) Get(id string) (*models.DBBackend, error) {
	var data string
	var sqlGet, args, err = sq.Select("backend").From("pad").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	err = db.Conn.QueryRow(sqlGet, args...).Scan(&data)
	if err != nil {
		return nil, err
	}

	var actualData = models.DBBackend{
		Backend: data,
	}

	return &actualData, nil
}

func (db *Postgres) CleanUpPads(padIds []string, padPrefix string) error {
	sqlDelete, args, err := sq.Delete("pad").Where(sq.And{sq.NotEq{"id": padIds},
		sq.Like{"backend": padPrefix}}).ToSql()
	if err != nil {
		return err
	}

	_, err = db.Conn.Exec(sqlDelete, args...)
	return err
}

func (db *Postgres) RecordClash(id string, data string) error {
	_, err := db.Conn.Exec("INSERT OR REPLACE INTO clashes (id, data) VALUES (?, ?)", id, data)
	if err != nil {
		return err
	}
	return nil
}

func (db *Postgres) Set(id string, dbModel models.DBBackend) error {

	_, err := db.Conn.Exec("INSERT OR REPLACE INTO pad (id, backend) VALUES (?, ?)", id, dbModel.Backend)
	if err != nil {
		return err
	}
	return nil
}

func (db *Postgres) GetAllPads() (map[string]string, error) {
	var padIDMap = make(map[string]string)
	var sqlGet, args, err = sq.Select("id, backend").From("pad").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var padID string
		var backend string
		if err := rows.Scan(&padID, &backend); err != nil {
			return nil, err
		}
		padIDMap[padID] = backend
	}

	return padIDMap, nil
}

func (db *Postgres) GetClashByPadID(padId string) ([]string, error) {
	var sqlGet, args, err = sq.Select("data").From("clashes").Where(sq.Eq{"id": padId}).ToSql()

	if err != nil {
		return nil, err
	}

	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var data = make([]string, 0)
	for rows.Next() {
		var clash string
		if err := rows.Scan(&clash); err != nil {
			return nil, err
		}
		data = append(data, clash)
	}

	return data, nil
}
