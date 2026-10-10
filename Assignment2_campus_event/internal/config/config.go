// Package config reads settings from environment variables.
//
// Nothing that differs between environments (ports, hostnames, database
// credentials) is hard-coded in the services: it all comes from here.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// MustGet returns the value of a required environment variable and stops
// the program with a clear message if it is missing.
func MustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: required environment variable %s is not set", key)
	}
	return v
}

// GetDuration reads an optional duration such as "2s" or "200ms".
func GetDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("config: %s=%q is not a valid duration: %v", key, v, err)
	}
	return d
}

// GetInt reads an optional integer setting.
func GetInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("config: %s=%q is not a valid integer: %v", key, v, err)
	}
	return n
}

// PostgresDSN builds a connection string from the DB_* variables.
func PostgresDSN() string {
	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		MustGet("DB_HOST"), MustGet("DB_PORT"), MustGet("DB_USER"),
		MustGet("DB_PASSWORD"), MustGet("DB_NAME"), sslMode)
}
