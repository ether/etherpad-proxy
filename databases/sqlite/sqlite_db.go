package sqlite

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	Conn *sql.DB
	sb   sq.StatementBuilderType
}

var _ interfaces.IDB = (*DB)(nil)

func NewSQLiteDB(filename string) (*DB, error) {
	conn, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Conn: conn,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}

	if _, err = db.Conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT, backend TEXT, PRIMARY KEY (id))"); err != nil {
		return nil, err
	}
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS clashes (id TEXT, data TEXT, PRIMARY KEY (id, data))"); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}

func (db *DB) Get(id string) (*models.DBBackend, error) {
	sqlGet, args, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, err
	}
	var data string
	if err = db.Conn.QueryRow(sqlGet, args...).Scan(&data); err != nil {
		return nil, err
	}
	return &models.DBBackend{Backend: data}, nil
}

func (db *DB) CleanUpPads(padIds []string, padPrefix string) error {
	sqlDelete, args, err := db.sb.Delete("pad").
		Where(sq.And{sq.NotEq{"id": padIds}, sq.Like{"backend": padPrefix}}).ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlDelete, args...)
	return err
}

func (db *DB) RecordClash(id string, data string) error {
	sqlStr, args, err := db.sb.Insert("clashes").Columns("id", "data").Values(id, data).
		Suffix("ON CONFLICT (id, data) DO NOTHING").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *DB) Set(id string, dbModel models.DBBackend) error {
	sqlStr, args, err := db.sb.Insert("pad").Columns("id", "backend").Values(id, dbModel.Backend).
		Suffix("ON CONFLICT (id) DO UPDATE SET backend = EXCLUDED.backend").ToSql()
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec(sqlStr, args...)
	return err
}

func (db *DB) Assign(padId string, candidate string) (string, error) {
	insSQL, insArgs, err := db.sb.Insert("pad").Columns("id", "backend").Values(padId, candidate).
		Suffix("ON CONFLICT (id) DO NOTHING").ToSql()
	if err != nil {
		return "", err
	}
	if _, err = db.Conn.Exec(insSQL, insArgs...); err != nil {
		return "", err
	}
	selSQL, selArgs, err := db.sb.Select("backend").From("pad").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return "", err
	}
	var backend string
	if err = db.Conn.QueryRow(selSQL, selArgs...).Scan(&backend); err != nil {
		return "", err
	}
	return backend, nil
}

func (db *DB) GetAllPads() (map[string]string, error) {
	padIDMap := make(map[string]string)
	sqlGet, args, err := db.sb.Select("id", "backend").From("pad").ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var padID, backend string
		if err := rows.Scan(&padID, &backend); err != nil {
			return nil, err
		}
		padIDMap[padID] = backend
	}
	return padIDMap, rows.Err()
}

func (db *DB) GetClashByPadID(padId string) ([]string, error) {
	sqlGet, args, err := db.sb.Select("data").From("clashes").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := db.Conn.Query(sqlGet, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	data := make([]string, 0)
	for rows.Next() {
		var clash string
		if err := rows.Scan(&clash); err != nil {
			return nil, err
		}
		data = append(data, clash)
	}
	return data, rows.Err()
}
