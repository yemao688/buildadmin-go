package cron

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Cron struct {
	server     *cron.Cron
	logger     *zap.Logger
	sqlDB      *gorm.DB
	exampleJob *ExampleJob
}

// NewCron .
func NewCron(sqlDB *gorm.DB, logger *zap.Logger, exampleJob *ExampleJob) *Cron {
	server := cron.New(
		cron.WithSeconds(),
	)

	return &Cron{
		logger: logger,
		sqlDB:  sqlDB,
		server: server,

		exampleJob: exampleJob,
	}
}

func (c *Cron) Run() error {
	// cron example
	//if _, err := c.server.AddFunc("*/5 * * * * *", c.exampleJob.Hello); err != nil {
	//   return err
	//}

	c.server.Start()
	return nil
}

func (c *Cron) Stop(ctx context.Context) error {
	c.server.Stop()
	return nil
}
