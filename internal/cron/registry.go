package cron

import (
	"fmt"
	"sync"
)

// Job 描述一个定时任务：Spec 为 cron 表达式（带秒），Fn 为任务函数。
type Job struct {
	Spec string
	Fn   func()
}

var (
	mu       sync.Mutex
	registry []Job
	frozen   bool
)

// Register 自注册一个定时任务，供 Cron.Run 启动时统一挂载。
// 业务仓库在自己的包内调用（通常放 init 或包级变量），无需改动框架的
// NewCron 签名、provider 或 wire——对齐 internal/migrations/business 的
// Register 模式，框架升级零冲突。
func Register(j Job) {
	mu.Lock()
	defer mu.Unlock()
	if frozen {
		panic("cron registry is frozen")
	}
	if j.Spec == "" || j.Fn == nil {
		panic("cron job requires spec and fn")
	}
	registry = append(registry, j)
}

// Jobs 返回已注册任务列表并冻结注册表（供 Cron.Run 在启动时消费）。
func Jobs() ([]Job, error) {
	mu.Lock()
	defer mu.Unlock()
	frozen = true
	list := append([]Job(nil), registry...)
	for i, job := range list {
		if job.Spec == "" || job.Fn == nil {
			return nil, fmt.Errorf("invalid cron job at index %d", i)
		}
	}
	return list, nil
}
