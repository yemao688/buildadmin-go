package handler

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestScheduleProcessExitUsesZeroExitCode(t *testing.T) {
	exited := make(chan int, 1)
	timer := scheduleProcessExit(zap.NewNop(), time.Millisecond, func(code int) {
		exited <- code
	})
	if timer == nil {
		t.Fatal("scheduleProcessExit returned nil timer")
	}
	t.Cleanup(func() { timer.Stop() })

	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled process exit did not run")
	}
}
