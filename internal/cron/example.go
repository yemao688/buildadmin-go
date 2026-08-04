package cron

import "go.uber.org/zap"

// ExampleJob 是定时任务的注册示例：业务包内 init 调用
// cron.Register(cron.Job{Spec: "*/5 * * * * *", Fn: job.Hello}) 即可接入，
// 无需改动框架的 NewCron/provider/wire。
type ExampleJob struct {
	logger *zap.Logger
}

func NewExampleJob(logger *zap.Logger) *ExampleJob {
	return &ExampleJob{
		logger: logger,
	}
}

func (j *ExampleJob) Hello() {
	j.logger.Info("hello")
}

func init() {
	// 注册示例（默认注释掉，避免业务仓库启动即打印 hello）：
	// Register(Job{Spec: "*/5 * * * * *", Fn: NewExampleJob(zap.L()).Hello})
}
