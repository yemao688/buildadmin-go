package handler

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations"
	"go-build-admin/database/migrations/model"
	"go-build-admin/service/db"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MigrateHandler struct {
	logger *zap.Logger
	db     *gorm.DB
}

func NewMigrateHandler(logger *zap.Logger, config *conf.Configuration) *MigrateHandler {
	return &MigrateHandler{
		logger: logger,
		db:     db.NewDB(config, logger),
	}
}

func (h *MigrateHandler) Run(cmd *cobra.Command, args []string) {

	err := h.db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&model.AdminGroupAccess{},
		&model.AdminGroup{},
		&model.AdminLog{},
		&model.AdminRule{},
		&model.Admin{},
		&model.Area{},
		&model.Attachment{},
		&model.Captcha{},
		&model.Config{},
		&model.CrudLog{},
		&model.Migration{},
		&model.SecurityDataRecycleLog{},
		&model.SecurityDataRecycle{},
		&model.SecuritySensitiveDataLog{},
		&model.SecuritySensitiveData{},
		&model.TestBuild{},
		&model.Token{},
		&model.UserGroup{},
		&model.UserMoneyLog{},
		&model.UserRule{},
		&model.UserScoreLog{},
		&model.User{},
	)
	if err != nil {
		cmd.Println("database migrate error:", err)
	}

	//插入数据
	install := migrations.NewInstall(h.db)
	install.InsertData()

	cmd.Println("database migrate success")
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
