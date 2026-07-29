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

func TestGenerationAdvisoryLockNameIncludesDatabaseAndPrefix(t *testing.T) {
	name := generationAdvisoryLockName("buildadmin_go", "ba_")
	if name != "buildadmin:crud:buildadmin_go:ba_" {
		t.Fatalf("name = %q", name)
	}
	long := generationAdvisoryLockName("database-with-a-very-long-name", "prefix-with-a-very-long-name-that-would-exceed-mysql-lock-limit")
	if len(long) > 64 {
		t.Fatalf("long lock name length = %d", len(long))
	}
}
