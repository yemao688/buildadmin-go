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
	config *conf.Configuration
}

func NewMigrateHandler(logger *zap.Logger, config *conf.Configuration) *MigrateHandler {
	return &MigrateHandler{
		logger: logger,
		db:     db.NewDB(config, logger),
		config: config,
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
		&model.Migrations{},
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
		return
	}

	// 版本化迁移（处理 AutoMigrate 无法完成的列类型变更）
	upgradeCount, err := migrations.RunVersionMigrations(h.db, h.config)
	if err != nil {
		cmd.Println("version migration error:", err)
		return
	}
	if upgradeCount > 0 {
		cmd.Printf("executed %d version migrations\n", upgradeCount)
	}

	//插入数据
	install := migrations.NewInstall(h.db)
	install.InsertData()
	cmd.Println("data seed completed")

	cmd.Println("database migrate success")
}

func (h *MigrateHandler) Rollback(cmd *cobra.Command, args []string) {
	//TODO:
}
