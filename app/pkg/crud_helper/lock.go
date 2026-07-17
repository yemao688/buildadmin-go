package crud_helper

import (
	"errors"
	"sync"
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
