package models

import (
	"database/sql"
	"time"

	"core/conf"
)

func configureConnectionPool(db *sql.DB, config conf.DBConfig) {
	maxOpen, maxIdle := config.MaxOpenConns, config.MaxIdleConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	if maxIdle <= 0 {
		maxIdle = 10
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	lifetime, idleTime := config.ConnMaxLifetimeSeconds, config.ConnMaxIdleTimeSeconds
	if lifetime <= 0 {
		lifetime = 1800
	}
	if idleTime <= 0 {
		idleTime = 300
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(lifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(idleTime) * time.Second)
}
