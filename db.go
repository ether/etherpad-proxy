package main

import (
	"database/sql"
	"github.com/ether/etherpad-proxy/models"
	_ "modernc.org/sqlite"
)

import sq "github.com/Masterminds/squirrel"

type DB struct {
	Conn *sql.DB
}

func NewDB(filename string) (*DB, error) {
	conn, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Conn: conn,
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

func (db *DB) CleanUpPads(padIds []string, padPrefix string) error {
	sqlDelete, args, err := sq.Delete("pad").Where(sq.And{sq.NotEq{"id": padIds},
		sq.Like{"backend": padPrefix}}).ToSql()
	if err != nil {
		return err
	}

	_, err = db.Conn.Exec(sqlDelete, args...)
	return err
}

func (db *DB) RecordClash(id string, data string) error {
	_, err := db.Conn.Exec("INSERT OR REPLACE INTO clashes (id, data) VALUES (?, ?)", id, data)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) Set(id string, dbModel models.DBBackend) error {

	_, err := db.Conn.Exec("INSERT OR REPLACE INTO pad (id, backend) VALUES (?, ?)", id, dbModel.Backend)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) getAllPads() (map[string]string, error) {
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

func (db *DB) getClashByPadID(padId string) ([]string, error) {
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
