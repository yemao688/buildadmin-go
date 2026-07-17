package crud_helper

import (
	"strings"
	"testing"
)

func TestTryAcquireGenerationLockBusyAndReusable(t *testing.T) {
	release, err := TryAcquireGenerationLock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TryAcquireGenerationLock(); err == nil || !strings.Contains(err.Error(), "another generation is in progress") {
		t.Fatalf("busy error = %v", err)
	}
	release()
	releaseAgain, err := TryAcquireGenerationLock()
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
