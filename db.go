package main

import (
	"database/sql"
	"encoding/json"
	"github.com/ether/etherpad-proxy/models"
	_ "modernc.org/sqlite"
)

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
	if _, err = db.Conn.Exec("CREATE TABLE IF NOT EXISTS pad (id TEXT, data TEXT)"); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}

func (db *DB) Get(id string) (*models.DBBackend, error) {
	var data string
	err := db.Conn.QueryRow("SELECT data FROM pad WHERE id = ?", id).Scan(&data)
	if err != nil {
		return nil, err
	}

	var actualData models.DBBackend

	if err = json.Unmarshal([]byte(data), &actualData); err != nil {
		return nil, err
	}

	return &actualData, nil
}

func (db *DB) Set(id string, dbModel models.DBBackend) error {
	data, err := json.Marshal(&dbModel)
	if err != nil {
		return err
	}
	_, err = db.Conn.Exec("INSERT OR REPLACE INTO pad (id, data) VALUES (?, ?)", id, string(data))
	if err != nil {
		return err
	}
	return nil
}
