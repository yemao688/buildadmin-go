package cron

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterAndJobs(t *testing.T) {
	registry = nil
	frozen = false

	var calls int32
	Register(Job{Spec: "*/5 * * * * *", Fn: func() { atomic.AddInt32(&calls, 1) }})
	Register(Job{Spec: "0 * * * * *", Fn: func() {}})

	jobs, err := Jobs()
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, "*/5 * * * * *", jobs[0].Spec)

	// 冻结后禁止继续注册（与 migrations/business 语义一致）
	require.Panics(t, func() {
		Register(Job{Spec: "* * * * * *", Fn: func() {}})
	})

	// 非法任务拒绝
	registry = nil
	frozen = false
	require.Panics(t, func() {
		Register(Job{Spec: "", Fn: func() {}})
	})
	require.Panics(t, func() {
		Register(Job{Spec: "* * * * * *"})
	})
}
