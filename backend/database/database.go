package database

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Retry configuration parameters
const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
	backoffFactor  = 2.0
	jitterFactor   = 0.2
)

// DB represents the database connection
type DB struct {
	*sqlx.DB
}

// New creates a new database connection with retry mechanism
func New(url string) (*DB, error) {
	if url == "" {
		return nil, errors.New("database URL must be provided")
	}

	slog.Info("Establishing database connection")

	var db *sqlx.DB
	var err error
	attempt := 1
	backoff := initialBackoff

	for {
		slog.Info("Attempting to connect to PostgreSQL", slog.Int("attempt", attempt))

		db, err = sqlx.Connect("postgres", url)
		if err != nil {
			slog.Error("Failed to connect to database", slog.String("error", err.Error()), slog.Int("attempt", attempt))

			time.Sleep(calculateBackoff(backoff))
			backoff = min(time.Duration(float64(backoff)*backoffFactor), maxBackoff)
			attempt++

			continue
		}

		// Configure connection pooling
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(5 * time.Minute)
		db.SetConnMaxIdleTime(1 * time.Minute)

		// Test connection with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr := db.PingContext(ctx)
		cancel()

		if pingErr != nil {
			slog.Error("Failed to ping database", slog.String("pingErr", pingErr.Error()), slog.Int("attempt", attempt))

			db.Close() // Close failed connection

			time.Sleep(calculateBackoff(backoff))
			backoff = min(time.Duration(float64(backoff)*backoffFactor), maxBackoff)
			attempt++

			continue
		}

		// Connection successful
		slog.Info("Successfully connected to PostgreSQL", slog.Int("attempt", attempt))
		return &DB{db}, nil
	}
}

// calculateBackoff adds jitter to avoid the thundering herd problem
func calculateBackoff(backoff time.Duration) time.Duration {
	jitter := float64(backoff) * jitterFactor
	return backoff + time.Duration(rand.Float64()*jitter)
}

// CheckConnection verifies the database connection is still alive and attempts to reconnect if needed
func (db *DB) CheckConnection(url string) error {
	if db.DB == nil {
		newDB, err := New(url)
		if err != nil {
			return err
		}
		*db = *newDB
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := db.PingContext(ctx)
	cancel()

	if err != nil {
		slog.Warn("Database connection lost, trying to reconnect", slog.String("error", err.Error()))
		db.Close() // Close problematic connection

		newDB, err := New(url)
		if err != nil {
			return err
		}
		*db = *newDB
	}

	return nil
}

// GetContext retrieves a single row and scans it into dest
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.DB.GetContext(ctx, dest, query, args...)
}

// SelectContext retrieves multiple rows and scans them into dest
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.DB.SelectContext(ctx, dest, query, args...)
}

// ExecContext executes a query without returning any rows
// func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sqlx.Result, error) {
// 	return db.DB.NamedExecContext(ctx, query, args...)
// }

// Transaction executes a function within a transaction
func (db *DB) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
