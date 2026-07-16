// Package requesttx carries an optional request-scoped GORM transaction.
//
// Bind is intended for middleware which already started a transaction. Model
// code should use Transaction so nested work reuses that transaction without
// committing it. WithDB is used internally when a model has only its normal
// connection available and preserves the non-transactional fallback path.
package requesttx

import (
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
)

type stateKey struct{}

type state struct {
	mu       sync.Mutex
	parent   context.Context
	db       *gorm.DB
	active   bool
	outcome  *Outcome
	finished bool
}

// Outcome is the transport-neutral response captured while a request
// transaction is active.
type Outcome struct {
	HTTPCode     int
	BusinessCode int
	Message      string
	Data         any
}

// Bind associates an already-started request transaction with ctx.
func Bind(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, stateKey{}, &state{parent: ctx, db: tx, active: true})
}

// WithDB associates a normal database connection for Transaction fallback.
// It does not make the request transaction active.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	if s := get(ctx); s != nil {
		return ctx
	}
	return context.WithValue(ctx, stateKey{}, &state{db: db})
}

// DB returns the request transaction or fallback connection, if bound.
func DB(ctx context.Context) *gorm.DB {
	if s := get(ctx); s != nil {
		return s.db
	}
	return nil
}

// Active reports whether ctx carries a request transaction.
func Active(ctx context.Context) bool {
	s := get(ctx)
	return s != nil && s.active && s.db != nil
}

// Transaction reuses an active request transaction without committing it.
// Without one, it runs the callback through the bound DB's Transaction API.
func Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	if fn == nil {
		return errors.New("requesttx: nil transaction callback")
	}
	if s := get(ctx); s != nil && s.active {
		if s.db == nil {
			return errors.New("requesttx: active transaction has no database")
		}
		return fn(s.db)
	}
	db := DB(ctx)
	if db == nil {
		return errors.New("requesttx: no database bound to context")
	}
	return db.Transaction(fn)
}

// Stage stores one response while a request transaction is active. It returns
// false when the request is non-transactional or already has an outcome.
func Stage(ctx context.Context, outcome Outcome) bool {
	s := get(ctx)
	if s == nil || !s.active {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outcome != nil || s.finished {
		return false
	}
	s.outcome = &outcome
	return true
}

// TakeOutcome consumes the staged response after the transaction is committed.
func TakeOutcome(ctx context.Context) (Outcome, bool) {
	s := get(ctx)
	if s == nil {
		return Outcome{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outcome == nil || s.finished {
		return Outcome{}, false
	}
	out := *s.outcome
	s.outcome = nil
	s.finished = true
	return out, true
}

// PeekOutcome returns the staged response without completing the request.
// Middleware uses it to decide whether the business handler succeeded before
// the surrounding transaction is committed.
func PeekOutcome(ctx context.Context) (Outcome, bool) {
	s := get(ctx)
	if s == nil {
		return Outcome{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.outcome == nil || s.finished {
		return Outcome{}, false
	}
	return *s.outcome, true
}

// DiscardOutcome clears a staged response when the transaction is rolled back.
func DiscardOutcome(ctx context.Context) {
	if s := get(ctx); s != nil {
		s.mu.Lock()
		s.outcome = nil
		s.finished = true
		s.mu.Unlock()
	}
}

// Finish closes a request transaction state. It intentionally clears the DB
// pointer so a handler retaining the bound context cannot use a finished tx.
func Finish(ctx context.Context) {
	if s := get(ctx); s != nil {
		s.mu.Lock()
		s.active = false
		s.db = nil
		s.finished = true
		s.mu.Unlock()
	}
}

// Unbind returns the context that was passed to Bind. Callers should restore
// the request context after Finish so subsequent middleware cannot observe a
// finished transaction state.
func Unbind(ctx context.Context) context.Context {
	if s := get(ctx); s != nil && s.parent != nil {
		return s.parent
	}
	return ctx
}

func get(ctx context.Context) *state {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(stateKey{}).(*state)
	return s
}
