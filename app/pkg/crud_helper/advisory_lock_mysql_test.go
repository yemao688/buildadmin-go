package crud_helper

import (
	"go-build-admin/app/pkg/advisorylock"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCrudAdvisoryLockBlocksAnotherSession(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db1, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db2, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
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
