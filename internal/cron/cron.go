package cron

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Cron struct {
	server *cron.Cron
	logger *zap.Logger
}

// NewCron .
func NewCron(logger *zap.Logger) *Cron {
	server := cron.New(
		cron.WithSeconds(),
	)

	return &Cron{
		logger: logger,
		server: server,
	}
}

func (c *Cron) Run() error {
	// 启动时挂载业务自注册任务（cron.Register），业务零改动接入：
	// 业务包内 init 调用 cron.Register(cron.Job{Spec: "...", Fn: ...}) 即可，
	// 无需修改本文件、provider 或 wire。
	jobs, err := Jobs()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if _, err := c.server.AddFunc(job.Spec, job.Fn); err != nil {
			c.logger.Error("cron job register failed", zap.String("spec", job.Spec), zap.Error(err))
			return err
		}
		c.logger.Info("cron job registered", zap.String("spec", job.Spec))
	}

	c.server.Start()
	return nil
}

func (c *Cron) Stop(ctx context.Context) error {
	c.server.Stop()
	return nil
}
