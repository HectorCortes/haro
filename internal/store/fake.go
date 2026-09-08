package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/HectorCortes/haro/internal/claim"
)

// FakeStore is an in-memory Store implementing all repositories without importing database/sql.
// It enforces CHECK/UNIQUE/FK via typed sentinels, monotonic cursors, RFC3339 UTC timestamps,
// and copy-on-write WithTx.
type FakeStore struct {
	mu   sync.Mutex
	inTx bool

	projects               map[string]*Project
	executions             map[string]*Execution
	steps                  map[string]*ExecutionStep
	generations            map[string]*Generation
	attempts               map[string]*Attempt
	attemptEvents          map[string]map[int]*AttemptEvent
	transitionEvents       map[string]map[int]*StepTransitionEvent
	transports             map[string]*Transport
	pathClaims             map[int64]*PathClaim
	pathClaimNextID        int64
	leases                 map[string]*Lease
	interactions           map[string]*Interaction
	interactionsByAttemptKey map[string]string // key: attemptID + "\x00" + idempotencyKey -> interaction ID
}

// NewFakeStore creates a new fake store.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		projects:               make(map[string]*Project),
		executions:             make(map[string]*Execution),
		steps:                  make(map[string]*ExecutionStep),
		generations:            make(map[string]*Generation),
		attempts:               make(map[string]*Attempt),
		attemptEvents:          make(map[string]map[int]*AttemptEvent),
		transitionEvents:       make(map[string]map[int]*StepTransitionEvent),
		transports:             make(map[string]*Transport),
		pathClaims:             make(map[int64]*PathClaim),
		leases:                 make(map[string]*Lease),
		interactions:           make(map[string]*Interaction),
		interactionsByAttemptKey: make(map[string]string),
	}
}

func (s *FakeStore) clone() *FakeStore {
	c := NewFakeStore()
	c.inTx = true
	for k, v := range s.projects {
		cp := *v
		c.projects[k] = &cp
	}
	for k, v := range s.executions {
		cp := *v
		if v.DagHash != nil {
			dh := *v.DagHash
			cp.DagHash = &dh
		}
		if v.BaseCommit != nil {
			bc := *v.BaseCommit
			cp.BaseCommit = &bc
		}
		if v.EndedAt != nil {
			ea := *v.EndedAt
			cp.EndedAt = &ea
		}
		c.executions[k] = &cp
	}
	for k, v := range s.steps {
		cp := *v
		c.steps[k] = &cp
	}
	for k, v := range s.generations {
		cp := *v
		if v.InvalidatedAt != nil {
			ia := *v.InvalidatedAt
			cp.InvalidatedAt = &ia
		}
		if v.InvalidatedByStep != nil {
			ib := *v.InvalidatedByStep
			cp.InvalidatedByStep = &ib
		}
		c.generations[k] = &cp
	}
	for k, v := range s.attempts {
		cp := *v
		if v.EndedAt != nil {
			ea := *v.EndedAt
			cp.EndedAt = &ea
		}
		if v.TerminationReason != nil {
			tr := *v.TerminationReason
			cp.TerminationReason = &tr
		}
		if v.ResultDigest != nil {
			rd := *v.ResultDigest
			cp.ResultDigest = &rd
		}
		c.attempts[k] = &cp
	}
	for k, m := range s.attemptEvents {
		nm := make(map[int]*AttemptEvent)
		for ck, ev := range m {
			cp := *ev
			if ev.PayloadRef != nil {
				pr := *ev.PayloadRef
				cp.PayloadRef = &pr
			}
			if ev.Payload != nil {
				pl := *ev.Payload
				cp.Payload = &pl
			}
			nm[ck] = &cp
		}
		c.attemptEvents[k] = nm
	}
	for k, m := range s.transitionEvents {
		nm := make(map[int]*StepTransitionEvent)
		for ck, ev := range m {
			cp := *ev
			if ev.FromStatus != nil {
				fs := *ev.FromStatus
				cp.FromStatus = &fs
			}
			nm[ck] = &cp
		}
		c.transitionEvents[k] = nm
	}
	for k, v := range s.transports {
		cp := *v
		if v.NativeSessionID != nil {
			ns := *v.NativeSessionID
			cp.NativeSessionID = &ns
		}
		if v.ProtocolVersion != nil {
			pv := *v.ProtocolVersion
			cp.ProtocolVersion = &pv
		}
		c.transports[k] = &cp
	}
	for k, v := range s.pathClaims {
		cp := *v
		if v.ReleasedAt != nil {
			ra := *v.ReleasedAt
			cp.ReleasedAt = &ra
		}
		c.pathClaims[k] = &cp
	}
	c.pathClaimNextID = s.pathClaimNextID
	for k, v := range s.leases {
		cp := *v
		c.leases[k] = &cp
	}
	for k, v := range s.interactions {
		cp := *v
		if v.Decision != nil {
			d := *v.Decision
			cp.Decision = &d
		}
		if v.ResolvedAt != nil {
			ra := *v.ResolvedAt
			cp.ResolvedAt = &ra
		}
		c.interactions[k] = &cp
	}
	for k, v := range s.interactionsByAttemptKey {
		c.interactionsByAttemptKey[k] = v
	}
	return c
}

// Store interface
func (s *FakeStore) Projects() ProjectsRepository { return &fakeProjectsRepo{s: s} }
func (s *FakeStore) Executions() ExecutionsRepository { return &fakeExecutionsRepo{s: s} }
func (s *FakeStore) Steps() StepsRepository { return &fakeStepsRepo{s: s} }
func (s *FakeStore) Attempts() AttemptsRepository { return &fakeAttemptsRepo{s: s} }
func (s *FakeStore) Generations() GenerationsRepository { return &fakeGenerationsRepo{s: s} }
func (s *FakeStore) Events() EventsRepository { return &fakeEventsRepo{s: s} }
func (s *FakeStore) Transport() TransportRepository { return &fakeTransportRepo{s: s} }
func (s *FakeStore) PathClaims() PathClaimRepository { return &fakePathClaimRepo{s: s} }
func (s *FakeStore) Leases() LeaseRepository { return &fakeLeasesRepo{s: s} }
func (s *FakeStore) Interactions() InteractionRepository { return &fakeInteractionsRepo{s: s} }
func (s *FakeStore) Close() error { return nil }

func (s *FakeStore) WithTx(ctx context.Context, fn func(Store) error) error {
	if s.inTx {
		return fn(s)
	}
	s.mu.Lock()
	clone := s.clone()
	s.mu.Unlock()
	if err := fn(clone); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// commit: replace maps
	s.projects = clone.projects
	s.executions = clone.executions
	s.steps = clone.steps
	s.generations = clone.generations
	s.attempts = clone.attempts
	s.attemptEvents = clone.attemptEvents
	s.transitionEvents = clone.transitionEvents
	s.transports = clone.transports
	s.pathClaims = clone.pathClaims
	s.pathClaimNextID = clone.pathClaimNextID
	s.leases = clone.leases
	s.interactions = clone.interactions
	s.interactionsByAttemptKey = clone.interactionsByAttemptKey
	return nil
}

// helpers for validation
func isValidExecutionStatus(st string) bool { return st == "pending" || st == "running" || st == "completed" || st == "failed" }
func isValidWorkspaceMode(m string) bool { return m == "isolated" || m == "shared" }
func isValidStepType(t string) bool { return t == "command" || t == "agent" || t == "workflow" }
func isValidStepStatus(s string) bool { return s == "pending" || s == "running" || s == "completed" || s == "failed" || s == "skipped" }
func isValidAttemptStatus(s string) bool { return s == "running" || s == "completed" || s == "failed" || s == "cancelled" }
func isValidInteractionType(t string) bool { return t == "permission" || t == "question" }
func isValidInteractionStatus(s string) bool { return s == "pending" || s == "resolved" }

// fakeProjectsRepo
type fakeProjectsRepo struct{ s *FakeStore }

func (r *fakeProjectsRepo) Create(ctx context.Context, id, rootPath string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, exists := r.s.projects[id]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: projects.id", ErrUniqueViolation)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	r.s.projects[id] = &Project{ID: id, RootPath: rootPath, CreatedAt: now}
	return nil
}
func (r *fakeProjectsRepo) Get(ctx context.Context, id string) (*Project, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p, ok := r.s.projects[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

// fakeExecutionsRepo
type fakeExecutionsRepo struct{ s *FakeStore }

func (r *fakeExecutionsRepo) Create(ctx context.Context, e *Execution) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, exists := r.s.executions[e.ID]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: executions.id", ErrUniqueViolation)
	}
	if _, ok := r.s.projects[e.ProjectID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if !isValidExecutionStatus(e.Status) {
		return fmt.Errorf("%w: CHECK constraint failed: executions.status", ErrCheckViolation)
	}
	if !isValidWorkspaceMode(e.WorkspaceMode) {
		return fmt.Errorf("%w: CHECK constraint failed: executions.workspace_mode", ErrCheckViolation)
	}
	if e.StartedAt == "" {
		e.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	cp := *e
	if e.DagHash != nil {
		dh := *e.DagHash
		cp.DagHash = &dh
	}
	if e.BaseCommit != nil {
		bc := *e.BaseCommit
		cp.BaseCommit = &bc
	}
	if e.EndedAt != nil {
		ea := *e.EndedAt
		cp.EndedAt = &ea
	}
	r.s.executions[e.ID] = &cp
	return nil
}
func (r *fakeExecutionsRepo) Get(ctx context.Context, id string) (*Execution, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.executions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	if e.DagHash != nil {
		dh := *e.DagHash
		cp.DagHash = &dh
	}
	if e.BaseCommit != nil {
		bc := *e.BaseCommit
		cp.BaseCommit = &bc
	}
	if e.EndedAt != nil {
		ea := *e.EndedAt
		cp.EndedAt = &ea
	}
	return &cp, nil
}
func (r *fakeExecutionsRepo) UpdateStatus(ctx context.Context, id, status string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.executions[id]
	if !ok {
		return ErrNotFound
	}
	if !isValidExecutionStatus(status) {
		return fmt.Errorf("%w: CHECK constraint failed", ErrCheckViolation)
	}
	e.Status = status
	if status == "completed" || status == "failed" {
		if e.EndedAt == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			e.EndedAt = &now
		}
	}
	return nil
}

// fakeStepsRepo
type fakeStepsRepo struct{ s *FakeStore }

func stepKey(execID, stepID string) string { return execID + "\x00" + stepID }

func (r *fakeStepsRepo) Create(ctx context.Context, s *ExecutionStep) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(s.ExecutionID, s.StepID)
	if _, exists := r.s.steps[k]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed", ErrUniqueViolation)
	}
	if _, ok := r.s.executions[s.ExecutionID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if !isValidStepType(s.Type) {
		return fmt.Errorf("%w: CHECK constraint failed: execution_steps.type", ErrCheckViolation)
	}
	if !isValidStepStatus(s.Status) {
		return fmt.Errorf("%w: CHECK constraint failed: execution_steps.status", ErrCheckViolation)
	}
	if !isValidWorkspaceMode(s.WorkspaceMode) {
		return fmt.Errorf("%w: CHECK constraint failed: execution_steps.workspace_mode", ErrCheckViolation)
	}
	cp := *s
	r.s.steps[k] = &cp
	return nil
}
func (r *fakeStepsRepo) Get(ctx context.Context, execID, stepID string) (*ExecutionStep, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(execID, stepID)
	s, ok := r.s.steps[k]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *s
	return &cp, nil
}
func (r *fakeStepsRepo) List(ctx context.Context, execID string) ([]*ExecutionStep, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []*ExecutionStep
	for _, s := range r.s.steps {
		if s.ExecutionID == execID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *fakeStepsRepo) UpdateStatus(ctx context.Context, execID, stepID, status string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if !isValidStepStatus(status) {
		return fmt.Errorf("%w: CHECK constraint failed", ErrCheckViolation)
	}
	k := stepKey(execID, stepID)
	s, ok := r.s.steps[k]
	if !ok {
		return ErrNotFound
	}
	s.Status = status
	return nil
}
func (r *fakeStepsRepo) UpdateGeneration(ctx context.Context, execID, stepID string, gen int) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(execID, stepID)
	s, ok := r.s.steps[k]
	if !ok {
		return ErrNotFound
	}
	s.CurrentGeneration = gen
	return nil
}

// fakeAttemptsRepo
type fakeAttemptsRepo struct{ s *FakeStore }

func (r *fakeAttemptsRepo) Create(ctx context.Context, a *Attempt) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, exists := r.s.attempts[a.ID]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: attempts.id", ErrUniqueViolation)
	}
	if !isValidAttemptStatus(a.Status) {
		return fmt.Errorf("%w: CHECK constraint failed: attempts.status", ErrCheckViolation)
	}
	k := stepKey(a.ExecutionID, a.StepID)
	if _, ok := r.s.steps[k]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if _, ok := r.s.generations[a.GenerationID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if a.StartedAt == "" {
		a.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	cp := *a
	if a.EndedAt != nil {
		ea := *a.EndedAt
		cp.EndedAt = &ea
	}
	if a.TerminationReason != nil {
		tr := *a.TerminationReason
		cp.TerminationReason = &tr
	}
	if a.ResultDigest != nil {
		rd := *a.ResultDigest
		cp.ResultDigest = &rd
	}
	r.s.attempts[a.ID] = &cp
	return nil
}
func (r *fakeAttemptsRepo) Get(ctx context.Context, id string) (*Attempt, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	a, ok := r.s.attempts[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *a
	if a.EndedAt != nil {
		ea := *a.EndedAt
		cp.EndedAt = &ea
	}
	if a.TerminationReason != nil {
		tr := *a.TerminationReason
		cp.TerminationReason = &tr
	}
	if a.ResultDigest != nil {
		rd := *a.ResultDigest
		cp.ResultDigest = &rd
	}
	return &cp, nil
}
func (r *fakeAttemptsRepo) UpdateStatus(ctx context.Context, id, status string, endedAt *string, term *string, digest *string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if !isValidAttemptStatus(status) {
		return fmt.Errorf("%w: CHECK constraint failed", ErrCheckViolation)
	}
	a, ok := r.s.attempts[id]
	if !ok {
		return ErrNotFound
	}
	a.Status = status
	if endedAt == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		a.EndedAt = &now
	} else {
		ea := *endedAt
		a.EndedAt = &ea
	}
	if term != nil {
		tr := *term
		a.TerminationReason = &tr
	} else {
		a.TerminationReason = nil
	}
	if digest != nil {
		rd := *digest
		a.ResultDigest = &rd
	} else {
		a.ResultDigest = nil
	}
	return nil
}
func (r *fakeAttemptsRepo) CountByStep(ctx context.Context, execID, stepID string) (int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	n := 0
	for _, a := range r.s.attempts {
		if a.ExecutionID == execID && a.StepID == stepID {
			n++
		}
	}
	return n, nil
}

// fakeGenerationsRepo
type fakeGenerationsRepo struct{ s *FakeStore }

func (r *fakeGenerationsRepo) Create(ctx context.Context, g *Generation) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, exists := r.s.generations[g.ID]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: generations.id", ErrUniqueViolation)
	}
	// UNIQUE(exec,step,number)
	for _, ex := range r.s.generations {
		if ex.ExecutionID == g.ExecutionID && ex.StepID == g.StepID && ex.Number == g.Number {
			return fmt.Errorf("%w: UNIQUE constraint failed: generations", ErrUniqueViolation)
		}
	}
	k := stepKey(g.ExecutionID, g.StepID)
	if _, ok := r.s.steps[k]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if g.CreatedAt == "" {
		g.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	cp := *g
	if g.InvalidatedAt != nil {
		ia := *g.InvalidatedAt
		cp.InvalidatedAt = &ia
	}
	if g.InvalidatedByStep != nil {
		ib := *g.InvalidatedByStep
		cp.InvalidatedByStep = &ib
	}
	r.s.generations[g.ID] = &cp
	return nil
}
func (r *fakeGenerationsRepo) Get(ctx context.Context, id string) (*Generation, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	g, ok := r.s.generations[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *g
	if g.InvalidatedAt != nil {
		ia := *g.InvalidatedAt
		cp.InvalidatedAt = &ia
	}
	if g.InvalidatedByStep != nil {
		ib := *g.InvalidatedByStep
		cp.InvalidatedByStep = &ib
	}
	return &cp, nil
}
func (r *fakeGenerationsRepo) GetByNumber(ctx context.Context, execID, stepID string, number int) (*Generation, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, g := range r.s.generations {
		if g.ExecutionID == execID && g.StepID == stepID && g.Number == number {
			cp := *g
			if g.InvalidatedAt != nil {
				ia := *g.InvalidatedAt
				cp.InvalidatedAt = &ia
			}
			if g.InvalidatedByStep != nil {
				ib := *g.InvalidatedByStep
				cp.InvalidatedByStep = &ib
			}
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}
func (r *fakeGenerationsRepo) ListByStep(ctx context.Context, execID, stepID string) ([]*Generation, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []*Generation
	for _, g := range r.s.generations {
		if g.ExecutionID == execID && g.StepID == stepID {
			cp := *g
			if g.InvalidatedAt != nil {
				ia := *g.InvalidatedAt
				cp.InvalidatedAt = &ia
			}
			if g.InvalidatedByStep != nil {
				ib := *g.InvalidatedByStep
				cp.InvalidatedByStep = &ib
			}
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *fakeGenerationsRepo) InvalidateByStep(ctx context.Context, execID, stepID, invalidatedBy string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, g := range r.s.generations {
		if g.ExecutionID == execID && g.StepID == stepID && g.InvalidatedAt == nil {
			g.InvalidatedAt = &now
			g.InvalidatedByStep = &invalidatedBy
		}
	}
	return nil
}

// fakeEventsRepo
type fakeEventsRepo struct{ s *FakeStore }

func (r *fakeEventsRepo) CreateAttemptEvent(ctx context.Context, e *AttemptEvent) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.attempts[e.AttemptID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if r.s.attemptEvents[e.AttemptID] == nil {
		r.s.attemptEvents[e.AttemptID] = make(map[int]*AttemptEvent)
	}
	if _, exists := r.s.attemptEvents[e.AttemptID][e.Cursor]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: attempt_events", ErrUniqueViolation)
	}
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	cp := *e
	if e.PayloadRef != nil {
		pr := *e.PayloadRef
		cp.PayloadRef = &pr
	}
	if e.Payload != nil {
		pl := *e.Payload
		cp.Payload = &pl
	}
	r.s.attemptEvents[e.AttemptID][e.Cursor] = &cp
	return nil
}

// PriorOutputDelta returns the latest output_delta from a prior attempt of
// the same execution/step as attemptID, mirroring the SQLite ordering.
func (r *fakeEventsRepo) PriorOutputDelta(ctx context.Context, attemptID string) (*AttemptEvent, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	cur, ok := r.s.attempts[attemptID]
	if !ok {
		return nil, ErrNotFound
	}
	var latestAtt *Attempt
	for _, a := range r.s.attempts {
		if a.ExecutionID != cur.ExecutionID || a.StepID != cur.StepID || a.ID == attemptID {
			continue
		}
		if latestAtt == nil || a.StartedAt > latestAtt.StartedAt || (a.StartedAt == latestAtt.StartedAt && a.ID > latestAtt.ID) {
			latestAtt = a
		}
	}
	if latestAtt == nil {
		return nil, ErrNotFound
	}
	var latestEv *AttemptEvent
	for _, ev := range r.s.attemptEvents[latestAtt.ID] {
		if ev.EventType != "output_delta" {
			continue
		}
		if latestEv == nil || ev.Cursor > latestEv.Cursor {
			latestEv = ev
		}
	}
	if latestEv == nil {
		return nil, ErrNotFound
	}
	cp := *latestEv
	if latestEv.PayloadRef != nil {
		pr := *latestEv.PayloadRef
		cp.PayloadRef = &pr
	}
	if latestEv.Payload != nil {
		pl := *latestEv.Payload
		cp.Payload = &pl
	}
	return &cp, nil
}
func (r *fakeEventsRepo) CreateTransition(ctx context.Context, e *StepTransitionEvent) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(e.ExecutionID, e.StepID)
	if r.s.transitionEvents[k] == nil {
		r.s.transitionEvents[k] = make(map[int]*StepTransitionEvent)
	}
	if _, exists := r.s.transitionEvents[k][e.Cursor]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: step_transition_events", ErrUniqueViolation)
	}
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	cp := *e
	if e.FromStatus != nil {
		fs := *e.FromStatus
		cp.FromStatus = &fs
	}
	r.s.transitionEvents[k][e.Cursor] = &cp
	return nil
}
func (r *fakeEventsRepo) ListTransitions(ctx context.Context, execID, stepID string) ([]*StepTransitionEvent, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(execID, stepID)
	m := r.s.transitionEvents[k]
	var out []*StepTransitionEvent
	for _, ev := range m {
		cp := *ev
		if ev.FromStatus != nil {
			fs := *ev.FromStatus
			cp.FromStatus = &fs
		}
		out = append(out, &cp)
	}
	// sort by cursor (simple bubble)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Cursor < out[i].Cursor {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
func (r *fakeEventsRepo) NextAttemptCursor(ctx context.Context, attemptID string) (int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	m := r.s.attemptEvents[attemptID]
	max := -1
	for cur := range m {
		if cur > max {
			max = cur
		}
	}
	return max + 1, nil
}
func (r *fakeEventsRepo) NextTransitionCursor(ctx context.Context, execID, stepID string) (int, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(execID, stepID)
	m := r.s.transitionEvents[k]
	max := -1
	for cur := range m {
		if cur > max {
			max = cur
		}
	}
	return max + 1, nil
}

// fakeTransportRepo
type fakeTransportRepo struct{ s *FakeStore }

func (r *fakeTransportRepo) Put(ctx context.Context, t *Transport) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.attempts[t.AttemptID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if t.Extra == "" {
		t.Extra = "{}"
	}
	cp := *t
	if t.NativeSessionID != nil {
		ns := *t.NativeSessionID
		cp.NativeSessionID = &ns
	}
	if t.ProtocolVersion != nil {
		pv := *t.ProtocolVersion
		cp.ProtocolVersion = &pv
	}
	r.s.transports[t.AttemptID] = &cp
	return nil
}
func (r *fakeTransportRepo) Get(ctx context.Context, attemptID string) (*Transport, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	tr, ok := r.s.transports[attemptID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *tr
	if tr.NativeSessionID != nil {
		ns := *tr.NativeSessionID
		cp.NativeSessionID = &ns
	}
	if tr.ProtocolVersion != nil {
		pv := *tr.ProtocolVersion
		cp.ProtocolVersion = &pv
	}
	return &cp, nil
}

// fakePathClaimRepo
type fakePathClaimRepo struct{ s *FakeStore }

func (r *fakePathClaimRepo) Acquire(ctx context.Context, c PathClaim) (bool, *PathClaim, error) {
	if c.ProjectID == "" || c.LogicalPath == "" || c.Mode == "" || c.OwnerExecutionID == "" || c.OwnerStepID == "" {
		return false, nil, fmt.Errorf("missing claim fields")
	}
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, ok := r.s.projects[c.ProjectID]; !ok {
		return false, nil, fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	// Scan active
	var actives []PathClaim
	for _, pc := range r.s.pathClaims {
		if pc.ProjectID == c.ProjectID && pc.ReleasedAt == nil {
			actives = append(actives, *pc)
		}
	}
	for _, ex := range actives {
		if claim.Overlaps(ex.LogicalPath, c.LogicalPath) {
			if claim.ShouldBlock(ex.Mode, c.Mode, "block", false) {
				conf := ex
				return false, &conf, nil
			}
		}
	}
	r.s.pathClaimNextID++
	now := time.Now().UTC().Format(time.RFC3339)
	pc := &PathClaim{
		ID:               r.s.pathClaimNextID,
		ProjectID:        c.ProjectID,
		LogicalPath:      c.LogicalPath,
		Mode:             c.Mode,
		OwnerExecutionID: c.OwnerExecutionID,
		OwnerStepID:      c.OwnerStepID,
		AcquiredAt:       now,
		ReleasedAt:       nil,
	}
	r.s.pathClaims[pc.ID] = pc
	return true, nil, nil
}
func (r *fakePathClaimRepo) Release(ctx context.Context, projectID, logicalPath, ownerExecutionID, ownerStepID string) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, pc := range r.s.pathClaims {
		if pc.ProjectID == projectID && pc.LogicalPath == logicalPath && pc.OwnerExecutionID == ownerExecutionID && pc.OwnerStepID == ownerStepID && pc.ReleasedAt == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			pc.ReleasedAt = &now
			return nil
		}
	}
	return fmt.Errorf("release: no active claim")
}
func (r *fakePathClaimRepo) ListActive(ctx context.Context, projectID string) ([]PathClaim, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []PathClaim
	for _, pc := range r.s.pathClaims {
		if pc.ProjectID == projectID && pc.ReleasedAt == nil {
			out = append(out, *pc)
		}
	}
	// order by logical_path
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].LogicalPath < out[i].LogicalPath {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// fakeLeasesRepo
type fakeLeasesRepo struct{ s *FakeStore }

func (r *fakeLeasesRepo) Acquire(ctx context.Context, executionID, stepID, holder string) (int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(executionID, stepID)
	now := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	if existing, ok := r.s.leases[k]; ok {
		next := existing.FencingToken + 1
		existing.Holder = holder
		existing.FencingToken = next
		existing.AcquiredAt = now
		existing.ExpiresAt = expires
		return next, nil
	}
	r.s.leases[k] = &Lease{ExecutionID: executionID, StepID: stepID, Holder: holder, FencingToken: 1, AcquiredAt: now, ExpiresAt: expires}
	return 1, nil
}
func (r *fakeLeasesRepo) Renew(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(executionID, stepID)
	l, ok := r.s.leases[k]
	if !ok || l.Holder != holder || l.FencingToken != fencingToken {
		return ErrNotFound
	}
	l.ExpiresAt = time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	return nil
}
func (r *fakeLeasesRepo) Release(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(executionID, stepID)
	l, ok := r.s.leases[k]
	if !ok || l.Holder != holder || l.FencingToken != fencingToken {
		return ErrNotFound
	}
	l.ExpiresAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}
func (r *fakeLeasesRepo) Get(ctx context.Context, executionID, stepID string) (*Lease, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	k := stepKey(executionID, stepID)
	l, ok := r.s.leases[k]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *l
	return &cp, nil
}

// fakeInteractionsRepo
type fakeInteractionsRepo struct{ s *FakeStore }

func (r *fakeInteractionsRepo) Create(ctx context.Context, i *Interaction) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if _, exists := r.s.interactions[i.ID]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: interactions.id", ErrUniqueViolation)
	}
	if _, ok := r.s.attempts[i.AttemptID]; !ok {
		return fmt.Errorf("%w: FOREIGN KEY constraint failed", ErrForeignKeyViolation)
	}
	if !isValidInteractionType(i.Type) {
		return fmt.Errorf("%w: CHECK constraint failed: interactions.type", ErrCheckViolation)
	}
	if !isValidInteractionStatus(i.Status) {
		return fmt.Errorf("%w: CHECK constraint failed: interactions.status", ErrCheckViolation)
	}
	key := i.AttemptID + "\x00" + i.IdempotencyKey
	if _, exists := r.s.interactionsByAttemptKey[key]; exists {
		return fmt.Errorf("%w: UNIQUE constraint failed: interactions", ErrUniqueViolation)
	}
	cp := *i
	if i.Decision != nil {
		d := *i.Decision
		cp.Decision = &d
	}
	if i.ResolvedAt != nil {
		ra := *i.ResolvedAt
		cp.ResolvedAt = &ra
	}
	r.s.interactions[i.ID] = &cp
	r.s.interactionsByAttemptKey[key] = i.ID
	return nil
}
func (r *fakeInteractionsRepo) Get(ctx context.Context, id string) (*Interaction, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	it, ok := r.s.interactions[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *it
	if it.Decision != nil {
		d := *it.Decision
		cp.Decision = &d
	}
	if it.ResolvedAt != nil {
		ra := *it.ResolvedAt
		cp.ResolvedAt = &ra
	}
	return &cp, nil
}
func (r *fakeInteractionsRepo) Resolve(ctx context.Context, id, idempotencyKey, decision string) (*Interaction, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	it, ok := r.s.interactions[id]
	if !ok {
		return nil, ErrNotFound
	}
	if it.IdempotencyKey != idempotencyKey {
		return nil, ErrUniqueViolation
	}
	if it.Status == "resolved" {
		cp := *it
		if it.Decision != nil {
			d := *it.Decision
			cp.Decision = &d
		}
		if it.ResolvedAt != nil {
			ra := *it.ResolvedAt
			cp.ResolvedAt = &ra
		}
		return &cp, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	it.Status = "resolved"
	it.Decision = &decision
	it.ResolvedAt = &now
	cp := *it
	if it.Decision != nil {
		d := *it.Decision
		cp.Decision = &d
	}
	if it.ResolvedAt != nil {
		ra := *it.ResolvedAt
		cp.ResolvedAt = &ra
	}
	return &cp, nil
}
