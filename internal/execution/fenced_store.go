package execution

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HectorCortes/haro/internal/store"
)

// ErrStaleLease identifies a write attempted after the attempt lost its
// fencing lease. Callers should reopen the step and start a new attempt.
var ErrStaleLease = errors.New("stale lease")

// FencedStore is a Store view that validates the attempt's lease before every
// mutating repository operation. Reads remain available after a lease is lost
// so callers can inspect the stale attempt and produce recovery guidance.
type FencedStore struct {
	base         store.Store
	executionID  string
	stepID       string
	holder       string
	fencingToken int64
}

// NewFencedStore creates a write-fenced view of base for one attempt.
func NewFencedStore(base store.Store, executionID, stepID, holder string, fencingToken int64) *FencedStore {
	return &FencedStore{
		base:         base,
		executionID:  executionID,
		stepID:       stepID,
		holder:       holder,
		fencingToken: fencingToken,
	}
}

func (s *FencedStore) validate(ctx context.Context) error {
	if s.base == nil {
		return fmt.Errorf("%w: nil store", ErrStaleLease)
	}
	lease, err := s.base.Leases().Get(ctx, s.executionID, s.stepID)
	if err != nil {
		return fmt.Errorf("%w: lease unavailable: %v", ErrStaleLease, err)
	}
	expiresAt, err := time.Parse(time.RFC3339, lease.ExpiresAt)
	if err != nil || !expiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("%w: lease expired; reopen and rerun", ErrStaleLease)
	}
	if lease.Holder != s.holder || lease.FencingToken != s.fencingToken {
		return fmt.Errorf("%w: lease fenced by a newer holder; reopen and rerun", ErrStaleLease)
	}
	return nil
}

func (s *FencedStore) guard(ctx context.Context, fn func() error) error {
	if err := s.validate(ctx); err != nil {
		return err
	}
	return fn()
}

func (s *FencedStore) Projects() store.ProjectsRepository {
	return fencedProjects{base: s.base.Projects(), guard: s.guard}
}

func (s *FencedStore) Executions() store.ExecutionsRepository {
	return fencedExecutions{base: s.base.Executions(), guard: s.guard}
}

func (s *FencedStore) Steps() store.StepsRepository {
	return fencedSteps{base: s.base.Steps(), guard: s.guard}
}

func (s *FencedStore) Attempts() store.AttemptsRepository {
	return fencedAttempts{base: s.base.Attempts(), guard: s.guard}
}

func (s *FencedStore) Generations() store.GenerationsRepository {
	return fencedGenerations{base: s.base.Generations(), guard: s.guard}
}

func (s *FencedStore) Events() store.EventsRepository {
	return fencedEvents{base: s.base.Events(), guard: s.guard}
}

func (s *FencedStore) Transport() store.TransportRepository {
	return fencedTransport{base: s.base.Transport(), guard: s.guard}
}

func (s *FencedStore) PathClaims() store.PathClaimRepository {
	return fencedPathClaims{base: s.base.PathClaims(), guard: s.guard}
}

func (s *FencedStore) Leases() store.LeaseRepository {
	return fencedLeases{base: s.base.Leases(), guard: s.guard}
}

func (s *FencedStore) Interactions() store.InteractionRepository {
	return fencedInteractions{base: s.base.Interactions(), guard: s.guard}
}

func (s *FencedStore) WithTx(ctx context.Context, fn func(store.Store) error) error {
	if err := s.validate(ctx); err != nil {
		return err
	}
	return s.base.WithTx(ctx, func(tx store.Store) error {
		fencedTx := NewFencedStore(tx, s.executionID, s.stepID, s.holder, s.fencingToken)
		return fn(fencedTx)
	})
}

func (s *FencedStore) Close() error { return s.base.Close() }

type fencedGuard func(context.Context, func() error) error

type fencedProjects struct {
	base  store.ProjectsRepository
	guard fencedGuard
}

func (r fencedProjects) Create(ctx context.Context, id, rootPath string) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, id, rootPath) })
}
func (r fencedProjects) Get(ctx context.Context, id string) (*store.Project, error) {
	return r.base.Get(ctx, id)
}

type fencedExecutions struct {
	base  store.ExecutionsRepository
	guard fencedGuard
}

func (r fencedExecutions) Create(ctx context.Context, e *store.Execution) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, e) })
}
func (r fencedExecutions) Get(ctx context.Context, id string) (*store.Execution, error) {
	return r.base.Get(ctx, id)
}
func (r fencedExecutions) UpdateStatus(ctx context.Context, id, status string) error {
	return r.guard(ctx, func() error { return r.base.UpdateStatus(ctx, id, status) })
}

type fencedSteps struct {
	base  store.StepsRepository
	guard fencedGuard
}

func (r fencedSteps) Create(ctx context.Context, step *store.ExecutionStep) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, step) })
}
func (r fencedSteps) Get(ctx context.Context, executionID, stepID string) (*store.ExecutionStep, error) {
	return r.base.Get(ctx, executionID, stepID)
}
func (r fencedSteps) List(ctx context.Context, executionID string) ([]*store.ExecutionStep, error) {
	return r.base.List(ctx, executionID)
}
func (r fencedSteps) UpdateStatus(ctx context.Context, executionID, stepID, status string) error {
	return r.guard(ctx, func() error { return r.base.UpdateStatus(ctx, executionID, stepID, status) })
}
func (r fencedSteps) UpdateGeneration(ctx context.Context, executionID, stepID string, generation int) error {
	return r.guard(ctx, func() error { return r.base.UpdateGeneration(ctx, executionID, stepID, generation) })
}
func (r fencedSteps) SetPendingFeedback(ctx context.Context, executionID, stepID string, feedback *string) error {
	return r.guard(ctx, func() error { return r.base.SetPendingFeedback(ctx, executionID, stepID, feedback) })
}

type fencedAttempts struct {
	base  store.AttemptsRepository
	guard fencedGuard
}

func (r fencedAttempts) Create(ctx context.Context, attempt *store.Attempt) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, attempt) })
}
func (r fencedAttempts) Get(ctx context.Context, id string) (*store.Attempt, error) {
	return r.base.Get(ctx, id)
}
func (r fencedAttempts) UpdateStatus(ctx context.Context, id, status string, endedAt, reason, digest *string) error {
	return r.guard(ctx, func() error { return r.base.UpdateStatus(ctx, id, status, endedAt, reason, digest) })
}
func (r fencedAttempts) CountByStep(ctx context.Context, executionID, stepID string) (int, error) {
	return r.base.CountByStep(ctx, executionID, stepID)
}
func (r fencedAttempts) CurrentByStep(ctx context.Context, executionID, stepID string) (*store.Attempt, error) {
	return r.base.CurrentByStep(ctx, executionID, stepID)
}

type fencedGenerations struct {
	base  store.GenerationsRepository
	guard fencedGuard
}

func (r fencedGenerations) Create(ctx context.Context, generation *store.Generation) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, generation) })
}
func (r fencedGenerations) Get(ctx context.Context, id string) (*store.Generation, error) {
	return r.base.Get(ctx, id)
}
func (r fencedGenerations) GetByNumber(ctx context.Context, executionID, stepID string, number int) (*store.Generation, error) {
	return r.base.GetByNumber(ctx, executionID, stepID, number)
}
func (r fencedGenerations) InvalidateByStep(ctx context.Context, executionID, stepID, invalidatedBy string) error {
	return r.guard(ctx, func() error { return r.base.InvalidateByStep(ctx, executionID, stepID, invalidatedBy) })
}
func (r fencedGenerations) ListByStep(ctx context.Context, executionID, stepID string) ([]*store.Generation, error) {
	return r.base.ListByStep(ctx, executionID, stepID)
}

type fencedEvents struct {
	base  store.EventsRepository
	guard fencedGuard
}

func (r fencedEvents) CreateAttemptEvent(ctx context.Context, event *store.AttemptEvent) error {
	return r.guard(ctx, func() error { return r.base.CreateAttemptEvent(ctx, event) })
}
func (r fencedEvents) PriorOutputDelta(ctx context.Context, attemptID string) (*store.AttemptEvent, error) {
	return r.base.PriorOutputDelta(ctx, attemptID)
}
func (r fencedEvents) CreateTransition(ctx context.Context, event *store.StepTransitionEvent) error {
	return r.guard(ctx, func() error { return r.base.CreateTransition(ctx, event) })
}
func (r fencedEvents) ListTransitions(ctx context.Context, executionID, stepID string) ([]*store.StepTransitionEvent, error) {
	return r.base.ListTransitions(ctx, executionID, stepID)
}
func (r fencedEvents) NextAttemptCursor(ctx context.Context, attemptID string) (int, error) {
	return r.base.NextAttemptCursor(ctx, attemptID)
}
func (r fencedEvents) NextTransitionCursor(ctx context.Context, executionID, stepID string) (int, error) {
	return r.base.NextTransitionCursor(ctx, executionID, stepID)
}
func (r fencedEvents) ListAttemptEvents(ctx context.Context, attemptID string, sinceCursor, limit int) ([]*store.AttemptEvent, error) {
	return r.base.ListAttemptEvents(ctx, attemptID, sinceCursor, limit)
}

type fencedTransport struct {
	base  store.TransportRepository
	guard fencedGuard
}

func (r fencedTransport) Put(ctx context.Context, transport *store.Transport) error {
	return r.guard(ctx, func() error { return r.base.Put(ctx, transport) })
}
func (r fencedTransport) Get(ctx context.Context, attemptID string) (*store.Transport, error) {
	return r.base.Get(ctx, attemptID)
}

type fencedPathClaims struct {
	base  store.PathClaimRepository
	guard fencedGuard
}

func (r fencedPathClaims) Acquire(ctx context.Context, claim store.PathClaim) (bool, *store.PathClaim, error) {
	var acquired bool
	var conflict *store.PathClaim
	err := r.guard(ctx, func() error {
		var err error
		acquired, conflict, err = r.base.Acquire(ctx, claim)
		return err
	})
	return acquired, conflict, err
}
func (r fencedPathClaims) Release(ctx context.Context, projectID, logicalPath, executionID, stepID string) error {
	return r.guard(ctx, func() error { return r.base.Release(ctx, projectID, logicalPath, executionID, stepID) })
}
func (r fencedPathClaims) ListActive(ctx context.Context, projectID string) ([]store.PathClaim, error) {
	return r.base.ListActive(ctx, projectID)
}

type fencedLeases struct {
	base  store.LeaseRepository
	guard fencedGuard
}

func (r fencedLeases) Acquire(ctx context.Context, executionID, stepID, holder string) (int64, error) {
	return r.base.Acquire(ctx, executionID, stepID, holder)
}
func (r fencedLeases) Renew(ctx context.Context, executionID, stepID, holder string, token int64) error {
	return r.guard(ctx, func() error { return r.base.Renew(ctx, executionID, stepID, holder, token) })
}
func (r fencedLeases) Release(ctx context.Context, executionID, stepID, holder string, token int64) error {
	return r.guard(ctx, func() error { return r.base.Release(ctx, executionID, stepID, holder, token) })
}
func (r fencedLeases) Get(ctx context.Context, executionID, stepID string) (*store.Lease, error) {
	return r.base.Get(ctx, executionID, stepID)
}

type fencedInteractions struct {
	base  store.InteractionRepository
	guard fencedGuard
}

func (r fencedInteractions) Create(ctx context.Context, interaction *store.Interaction) error {
	return r.guard(ctx, func() error { return r.base.Create(ctx, interaction) })
}
func (r fencedInteractions) Get(ctx context.Context, id string) (*store.Interaction, error) {
	return r.base.Get(ctx, id)
}
func (r fencedInteractions) Resolve(ctx context.Context, id, idempotencyKey, decision string) (*store.Interaction, error) {
	var resolved *store.Interaction
	err := r.guard(ctx, func() error {
		var err error
		resolved, err = r.base.Resolve(ctx, id, idempotencyKey, decision)
		return err
	})
	return resolved, err
}
