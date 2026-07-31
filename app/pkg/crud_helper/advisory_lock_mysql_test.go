package crud_helper

import (
	"go-build-admin/app/pkg/advisorylock"
	"go-build-admin/app/pkg/testutil"
	"testing"
	"time"
)

func TestCrudAdvisoryLockBlocksAnotherSession(t *testing.T) {
	db1, _ := testutil.OpenMySQL(t)
	db2, _ := testutil.OpenMySQL(t)
	name := generationAdvisoryLockName("crud-test", "ba_lock_test_") + time.Now().Format("150405.000000")
	_, first, err := advisorylock.Acquire(db1, name, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	secondDone := make(chan error, 1)
	go func() {
		_, second, acquireErr := advisorylock.Acquire(db2, name, 100*time.Millisecond)
		if second != nil {
			_ = second.Close()
		}
		secondDone <- acquireErr
	}()
	if err := <-secondDone; err == nil {
		t.Fatal("second session acquired an active CRUD advisory lock")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	_, third, err := advisorylock.Acquire(db2, name, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}
