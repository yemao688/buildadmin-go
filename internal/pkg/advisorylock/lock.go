package advisorylock

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Handle owns the database connection that holds a MySQL advisory lock.
// RELEASE_LOCK must run on the same session as GET_LOCK.
type Handle struct {
	conn   *sql.Conn
	name   string
	closed bool
}

// Acquire obtains a MySQL advisory lock and returns a GORM session pinned to
// the connection that owns it.
func Acquire(db *gorm.DB, name string, timeout time.Duration) (*gorm.DB, *Handle, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is required")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return nil, nil, err
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(context.Background(), "SELECT GET_LOCK(?, ?)", name, int(timeout/time.Second)).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("could not acquire advisory lock %q", name)
	}
	pinned := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
	pinned.Statement.ConnPool = conn
	return pinned, &Handle{conn: conn, name: name}, nil
}

// Close releases the advisory lock and returns the connection to the pool.
func (h *Handle) Close() error {
	if h == nil || h.closed {
		return nil
	}
	h.closed = true
	var released sql.NullInt64
	err := h.conn.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", h.name).Scan(&released)
	if err == nil && (!released.Valid || released.Int64 != 1) {
		err = fmt.Errorf("advisory lock %q was not released", h.name)
	}
	closeErr := h.conn.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// With holds an advisory lock for the duration of fn. A callback error wins
// over a release error, matching the migration runner's existing contract.
func With(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) (err error) {
	pinned, handle, err := Acquire(db, name, timeout)
	if err != nil {
		return err
	}
	defer func() {
		releaseErr := handle.Close()
		if err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	return fn(pinned)
}

// ValidateRelease keeps the migration package's release predicate reusable.
func ValidateRelease(released sql.NullInt64) error {
	if !released.Valid || released.Int64 != 1 {
		return fmt.Errorf("advisory lock was not released")
	}
	return nil
}
