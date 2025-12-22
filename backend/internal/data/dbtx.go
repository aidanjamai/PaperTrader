package data

import (
	"database/sql"
)

// DBTX describes the common interface for *sql.DB and *sql.Tx
type DBTX interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

