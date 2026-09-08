package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Sentinels for normalized SQLite errors.
var (
	ErrNotFound             = errors.New("not found")
	ErrCheckViolation       = errors.New("check violation")
	ErrUniqueViolation      = errors.New("unique violation")
	ErrForeignKeyViolation  = errors.New("foreign key violation")
)

// normalizeSQLiteError maps sqlite errors to typed sentinels.
// Handles sql.ErrNoRows -> ErrNotFound, and constraint strings/codes.
// Codes: 275=CHECK, 787=FK, 1555/2067=UNIQUE.
func normalizeSQLiteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "check constraint failed") {
		return fmt.Errorf("%w: %s", ErrCheckViolation, msg)
	}
	if strings.Contains(msg, "275") && strings.Contains(lower, "check") {
		return fmt.Errorf("%w: %s", ErrCheckViolation, msg)
	}
	if strings.Contains(msg, "787") || strings.Contains(lower, "foreign key constraint failed") {
		return fmt.Errorf("%w: %s", ErrForeignKeyViolation, msg)
	}
	if strings.Contains(msg, "1555") || strings.Contains(msg, "2067") || strings.Contains(lower, "unique constraint failed") {
		return fmt.Errorf("%w: %s", ErrUniqueViolation, msg)
	}
	// Fallback string checks for modernc messages
	if strings.Contains(lower, "foreign key") {
		return fmt.Errorf("%w: %s", ErrForeignKeyViolation, msg)
	}
	if strings.Contains(lower, "unique") {
		return fmt.Errorf("%w: %s", ErrUniqueViolation, msg)
	}
	if strings.Contains(lower, "check") && strings.Contains(lower, "constraint") {
		return fmt.Errorf("%w: %s", ErrCheckViolation, msg)
	}
	if strings.Contains(lower, "no rows") || strings.Contains(lower, "no such") && strings.Contains(lower, "not found") {
		return ErrNotFound
	}
	return err
}

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

// Execution is a stored execution.
type Execution struct {
	ID              string
	ProjectID       string
	WorkflowSource  string
	Status          string
	WorkspaceMode   string
	WorkspaceRoot   string
	StartedAt       string
	EndedAt         *string
	DagHash         *string
	BaseCommit      *string
}

// ExecutionStep is a stored step.
type ExecutionStep struct {
	ExecutionID      string
	StepID           string
	Type             string
	Status           string
	DependsOn        string // JSON array
	Requires         string // JSON array
	Produces         string // JSON array
	WorkspaceMode    string
	CurrentGeneration int
}

// Generation is a stored generation.
type Generation struct {
	ID                 string
	ExecutionID        string
	StepID             string
	Number             int
	CreatedAt          string
	InvalidatedAt      *string
	InvalidatedByStep  *string
}

// Attempt is a stored attempt.
type Attempt struct {
	ID                string
	ExecutionID       string
	StepID            string
	GenerationID      string
	Status            string
	StartedAt         string
	EndedAt           *string
	TerminationReason *string
	ResultDigest      *string
}

// StepTransitionEvent is an audited transition.
type StepTransitionEvent struct {
	ID          int64
	ExecutionID string
	StepID      string
	Cursor      int
	FromStatus  *string
	ToStatus    string
	OccurredAt  string
}

// AttemptEvent is an attempt event.
type AttemptEvent struct {
	ID         int64
	AttemptID  string
	Cursor     int
	EventType  string
	PayloadRef *string
	// Payload holds an inline, sanitized, bounded evidence delta for
	// output_delta events. Non-nil wins over the legacy PayloadRef; it may
	// be empty and is never raw/full output.
	Payload    *string
	OccurredAt string
}

// ExecutionsRepository manages executions.
type ExecutionsRepository interface {
	Create(ctx context.Context, e *Execution) error
	Get(ctx context.Context, id string) (*Execution, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

// StepsRepository manages execution_steps.
type StepsRepository interface {
	Create(ctx context.Context, s *ExecutionStep) error
	Get(ctx context.Context, executionID, stepID string) (*ExecutionStep, error)
	List(ctx context.Context, executionID string) ([]*ExecutionStep, error)
	UpdateStatus(ctx context.Context, executionID, stepID, status string) error
	UpdateGeneration(ctx context.Context, executionID, stepID string, gen int) error
}

// AttemptsRepository manages attempts.
type AttemptsRepository interface {
	Create(ctx context.Context, a *Attempt) error
	Get(ctx context.Context, id string) (*Attempt, error)
	UpdateStatus(ctx context.Context, id, status string, endedAt *string, terminationReason *string, resultDigest *string) error
	CountByStep(ctx context.Context, executionID, stepID string) (int, error)
}

// GenerationsRepository manages generations.
type GenerationsRepository interface {
	Create(ctx context.Context, g *Generation) error
	Get(ctx context.Context, id string) (*Generation, error)
	GetByNumber(ctx context.Context, executionID, stepID string, number int) (*Generation, error)
	InvalidateByStep(ctx context.Context, executionID, stepID, invalidatedBy string) error
	ListByStep(ctx context.Context, executionID, stepID string) ([]*Generation, error)
}

// Transport is per-adapter details for an attempt.
type Transport struct {
	AttemptID       string
	AdapterName     string
	NativeSessionID *string
	ProtocolVersion *int
	Extra           string // JSON, default '{}'
}

// TransportRepository manages attempt_transport.
type TransportRepository interface {
	Put(ctx context.Context, t *Transport) error
	Get(ctx context.Context, attemptID string) (*Transport, error)
}

// EventsRepository manages attempt_events and step_transition_events.
type EventsRepository interface {
	CreateAttemptEvent(ctx context.Context, e *AttemptEvent) error
	// PriorOutputDelta returns the latest output_delta event from a prior
	// attempt of the same execution and step as attemptID, ordered by attempt
	// start time, then event cursor/id. ErrNotFound when no prior exists.
	PriorOutputDelta(ctx context.Context, attemptID string) (*AttemptEvent, error)
	CreateTransition(ctx context.Context, e *StepTransitionEvent) error
	ListTransitions(ctx context.Context, executionID, stepID string) ([]*StepTransitionEvent, error)
	NextAttemptCursor(ctx context.Context, attemptID string) (int, error)
	NextTransitionCursor(ctx context.Context, executionID, stepID string) (int, error)
}

// Lease is a fencing lease.
type Lease struct {
	ExecutionID  string
	StepID       string
	Holder       string
	FencingToken int64
	AcquiredAt   string
	ExpiresAt    string
}

// LeaseRepository manages leases with fencing tokens.
type LeaseRepository interface {
	Acquire(ctx context.Context, executionID, stepID, holder string) (int64, error)
	Renew(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
	Release(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error
	Get(ctx context.Context, executionID, stepID string) (*Lease, error)
}

// Interaction is a human interaction.
type Interaction struct {
	ID              string
	AttemptID       string
	Type            string
	Status          string
	Decision        *string
	IdempotencyKey  string
	ResolvedAt      *string
}

// InteractionRepository manages interactions with CAS.
type InteractionRepository interface {
	Create(ctx context.Context, i *Interaction) error
	Get(ctx context.Context, id string) (*Interaction, error)
	Resolve(ctx context.Context, id, idempotencyKey, decision string) (*Interaction, error)
}

// Store is the repository facade. Only internal/store imports database/sql.
type Store interface {
	Projects() ProjectsRepository
	Executions() ExecutionsRepository
	Steps() StepsRepository
	Attempts() AttemptsRepository
	Generations() GenerationsRepository
	Events() EventsRepository
	Transport() TransportRepository
	PathClaims() PathClaimRepository
	Leases() LeaseRepository
	Interactions() InteractionRepository
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
	dsn := fmt.Sprintf("file:%s?cache=shared&_pragma=foreign_keys(1)", path)
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

// Executions returns executions repo.
func (s *SQLiteStore) Executions() ExecutionsRepository { return &executionsRepo{store: s} }

// Steps returns steps repo.
func (s *SQLiteStore) Steps() StepsRepository { return &stepsRepo{store: s} }

// Attempts returns attempts repo.
func (s *SQLiteStore) Attempts() AttemptsRepository { return &attemptsRepo{store: s} }

// Generations returns generations repo.
func (s *SQLiteStore) Generations() GenerationsRepository { return &generationsRepo{store: s} }

// Events returns events repo.
func (s *SQLiteStore) Events() EventsRepository { return &eventsRepo{store: s} }

// Transport returns transport repo.
func (s *SQLiteStore) Transport() TransportRepository { return &transportRepo{store: s} }

// PathClaims returns path claims repo.
func (s *SQLiteStore) PathClaims() PathClaimRepository { return &pathClaimRepo{store: s} }

// Leases returns leases repo.
func (s *SQLiteStore) Leases() LeaseRepository { return &leasesRepo{store: s} }

// Interactions returns interactions repo.
func (s *SQLiteStore) Interactions() InteractionRepository { return &interactionsRepo{store: s} }

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

// QueryForTest exposes query for tests (uses underlying db/tx).
func (s *SQLiteStore) QueryForTest(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.query(ctx, query, args...)
}

// projectsRepo implements ProjectsRepository.
type projectsRepo struct {
	store *SQLiteStore
}

func (r *projectsRepo) Create(ctx context.Context, id, rootPath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.store.exec(ctx, "INSERT INTO projects(id, root_path, created_at) VALUES (?, ?, ?)", id, rootPath, now)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *projectsRepo) Get(ctx context.Context, id string) (*Project, error) {
	row := r.store.queryRow(ctx, "SELECT id, root_path, created_at FROM projects WHERE id = ?", id)
	var p Project
	if err := row.Scan(&p.ID, &p.RootPath, &p.CreatedAt); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return &p, nil
}
