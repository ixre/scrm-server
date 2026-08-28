package models

import (
	"database/sql"
	"testing"

	"core/conf"
)

func TestConnectionPoolIsBounded(t *testing.T) {
	db, err := sql.Open("pgx", "") // configuration only, no connection is opened
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	configureConnectionPool(db, conf.DBConfig{})
	if got := db.Stats().MaxOpenConnections; got != 25 {
		t.Fatalf("default max connections = %d", got)
	}
	configureConnectionPool(db, conf.DBConfig{MaxOpenConns: 7, MaxIdleConns: 20})
	if got := db.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("configured max connections = %d", got)
	}
}
