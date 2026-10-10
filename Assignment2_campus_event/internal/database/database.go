// Package database opens a PostgreSQL connection, waiting for the
// database container to become ready.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Connect opens the database and retries for up to ~60 seconds, because
// with Docker Compose the service can start before Postgres accepts
// connections.
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)

	for attempt := 1; attempt <= 30; attempt++ {
		if err = db.Ping(); err == nil {
			return db, nil
		}
		log.Printf("database not ready (attempt %d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("could not connect to database: %w", err)
}
