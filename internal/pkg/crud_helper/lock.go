package crud_helper

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"buildadmin-go/internal/pkg/advisorylock"
	"buildadmin-go/internal/conf"
	"sync"
	"time"

	"gorm.io/gorm"
)

var ErrGenerationBusy = errors.New("another generation is in progress")

var generationMu sync.Mutex

// TryAcquireGenerationLock serializes the complete CRUD file/schema operation.
// The returned function releases the lock and is safe to defer immediately.
func TryAcquireGenerationLock() (func(), error) {
	if !generationMu.TryLock() {
		return nil, ErrGenerationBusy
	}
	return generationMu.Unlock, nil
}

const generationAdvisoryLockTimeout = 120 * time.Second

// generationAdvisoryLockName is shared by generate, delete, apply and plan so
// a process cannot bypass another CRUD operation by choosing a different CLI
// entry point. MySQL limits GET_LOCK names to 64 bytes.
func generationAdvisoryLockName(database, prefix string) string {
	name := "buildadmin:crud:" + database + ":" + prefix
	if len(name) <= 64 {
		return name
	}
	digest := sha256.Sum256([]byte(name))
	return name[:55] + ":" + hex.EncodeToString(digest[:])[:8]
}

// acquireGenerationLocks keeps the old process-local fast rejection and adds
// a session-pinned MySQL advisory lock for multi-process deployments. SQLite
// and unit-test databases retain the local lock only.
func acquireGenerationLocks(db *gorm.DB, cfg *conf.Configuration) (*gorm.DB, func() error, error) {
	releaseLocal, err := TryAcquireGenerationLock()
	if err != nil {
		return nil, nil, err
	}
	release := func() error {
		releaseLocal()
		return nil
	}
	if db == nil || cfg == nil || db.Dialector.Name() != "mysql" {
		return db, release, nil
	}
	database := cfg.Database.Database
	if database == "" {
		if err := db.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil {
			releaseLocal()
			return nil, nil, err
		}
	}
	name := generationAdvisoryLockName(database, cfg.Database.Prefix)
	pinned, handle, err := advisorylock.Acquire(db, name, generationAdvisoryLockTimeout)
	if err != nil {
		releaseLocal()
		return nil, nil, fmt.Errorf("crud advisory lock: %w", err)
	}
	return pinned, func() error {
		advisoryErr := handle.Close()
		releaseLocal()
		return advisoryErr
	}, nil
}
