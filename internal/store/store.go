package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Project is a stored project.
type Project struct {
	ID        string
	RootPath  string
	CreatedAt string
}

// ProjectsRepository manages projects.
type ProjectsRepository interface {
	Create(ctx context.Context, id, rootPath string) error
	Get(ctx context.Context, id string) (*Project, error)
}

// ExecutionsRepository manages executions (stub for now).
type ExecutionsRepository interface{}

// StepsRepository manages execution_steps (stub).
type StepsRepository interface{}

// AttemptsRepository manages attempts (stub).
type AttemptsRepository interface{}

// GenerationsRepository manages generations (stub).
type GenerationsRepository interface{}

// EventsRepository manages attempt_events and step_transition_events (stub).
type EventsRepository interface{}

// Store is the repository facade. Only internal/store imports database/sql.
type Store interface {
	Projects() ProjectsRepository
	Executions() ExecutionsRepository
	Steps() StepsRepository
	Attempts() AttemptsRepository
	Generations() GenerationsRepository
	Events() EventsRepository
	WithTx(ctx context.Context, fn func(Store) error) error
	Close() error
}

// SQLiteStore is the SQLite implementation of Store.
type SQLiteStore struct {
	db *sql.DB
	tx *sql.Tx
}

// Open opens a SQLite database at path, creates schema, and sets pragmas.
// path may be a file path; for tests use t.TempDir() file.
func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?cache=shared", path)
	// modernc.org/sqlite uses URI; plain path also works but we use file: URI
	// to ensure shared cache works.
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Ensure we can reach DB.
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// Pragmas.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma journal_mode: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}



// Projects returns the projects repository.
func (s *SQLiteStore) Projects() ProjectsRepository {
	return &projectsRepo{store: s}
}

// Executions stub.
func (s *SQLiteStore) Executions() ExecutionsRepository { return stubExecRepo{} }

// Steps stub.
func (s *SQLiteStore) Steps() StepsRepository { return stubStepsRepo{} }

// Attempts stub.
func (s *SQLiteStore) Attempts() AttemptsRepository { return stubAttemptsRepo{} }

// Generations stub.
func (s *SQLiteStore) Generations() GenerationsRepository { return stubGenerationsRepo{} }

// Events stub.
func (s *SQLiteStore) Events() EventsRepository { return stubEventsRepo{} }

type stubExecRepo struct{}
type stubStepsRepo struct{}
type stubAttemptsRepo struct{}
type stubGenerationsRepo struct{}
type stubEventsRepo struct{}

// WithTx executes fn in a transaction. Nested calls reuse the outer transaction.
func (s *SQLiteStore) WithTx(ctx context.Context, fn func(Store) error) error {
	if s.tx != nil {
		// Already in transaction — reuse.
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	txStore := &SQLiteStore{db: s.db, tx: tx}
	if err := fn(txStore); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// exec handles both db and tx.
func (s *SQLiteStore) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.tx != nil {
		return s.tx.ExecContext(ctx, query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *SQLiteStore) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if s.tx != nil {
		return s.tx.QueryRowContext(ctx, query, args...)
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

// projectsRepo implements ProjectsRepository.
type projectsRepo struct {
	store *SQLiteStore
}

func (r *projectsRepo) Create(ctx context.Context, id, rootPath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.store.exec(ctx, "INSERT INTO projects(id, root_path, created_at) VALUES (?, ?, ?)", id, rootPath, now)
	return err
}

func (r *projectsRepo) Get(ctx context.Context, id string) (*Project, error) {
	row := r.store.queryRow(ctx, "SELECT id, root_path, created_at FROM projects WHERE id = ?", id)
	var p Project
	if err := row.Scan(&p.ID, &p.RootPath, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}
