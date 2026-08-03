package official

import (
	"encoding/json"
	"fmt"

	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/internal/core"
	"buildadmin-go/internal/model"

	"gorm.io/gorm"
)

func version206(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	configTable := core.TableName(config, "config")
	if core.TableExists(db, configTable) {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+core.QuoteIdentifier(configTable)+" WHERE "+core.QuoteIdentifier("name")+" = ?", "backend_entrance").Scan(&count).Error; err != nil {
			return fmt.Errorf("check backend_entrance config: %w", err)
		}
		if count == 0 {
			var configID int64
			if err := db.Raw("SELECT COALESCE(MAX(" + core.QuoteIdentifier("id") + "), 0) + 1 FROM " + core.QuoteIdentifier(configTable)).Scan(&configID).Error; err != nil {
				return fmt.Errorf("allocate backend_entrance config id: %w", err)
			}
			// Keep IDs 14-19 available for the framework upload-config seed.
			if configID < 20 {
				configID = 20
			}
			if err := db.Exec("INSERT INTO "+core.QuoteIdentifier(configTable)+" (`id`, `name`, `group`, `title`, `type`, `value`, `rule`, `weigh`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", configID, "backend_entrance", "basics", "Backend entrance", "string", "/admin", "required", 1).Error; err != nil {
				return fmt.Errorf("insert backend_entrance config: %w", err)
			}
		}
	}

	tables := []struct {
		name  string
		model any
	}{
		{"crud_log", &model.Log{}},
		{"security_data_recycle", &model.SecurityDataRecycle{}},
		{"security_data_recycle_log", &model.SecurityDataRecycleLog{}},
		{"security_sensitive_data", &model.SecuritySensitiveData{}},
		{"security_sensitive_data_log", &model.SecuritySensitiveDataLog{}},
	}
	for _, item := range tables {
		fullTable := core.TableName(config, item.name)
		migrator := db.Table(fullTable).Migrator()
		if !migrator.HasTable(item.model) {
			continue
		}
		if migrator.HasColumn(item.model, "Connection") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(fullTable) + " ADD COLUMN `connection` varchar(100) NOT NULL DEFAULT '' COMMENT '数据库连接配置标识'").Error; err != nil {
			return fmt.Errorf("add connection column to %s: %w", fullTable, err)
		}
	}
	return nil
}

func version222(db *gorm.DB, config *conf.Configuration) error {
	// 列类型扩容
	type alterSpec struct {
		table string
		model any
		field string
	}
	alterSpecs := []alterSpec{
		{"attachment", &upload.Attachment{}, "Name"},
		{"admin", &model.Admin{}, "Password"},
		{"user", &model.User{}, "Password"},
	}
	for _, spec := range alterSpecs {
		fullTable := core.TableName(config, spec.table)
		if err := db.Table(fullTable).Migrator().AlterColumn(spec.model, spec.field); err != nil {
			return fmt.Errorf("alter %s.%s: %w", fullTable, spec.field, err)
		}
	}

	// crud_log 新增 comment 和 sync（独立检查，幂等）
	crudLogTable := core.TableName(config, "crud_log")
	crudLogModel := &model.Log{}
	crudMigrator := db.Table(crudLogTable).Migrator()
	if !crudMigrator.HasColumn(crudLogModel, "Comment") {
		if err := crudMigrator.AddColumn(crudLogModel, "Comment"); err != nil {
			return fmt.Errorf("add comment column to %s: %w", crudLogTable, err)
		}
	}
	if !crudMigrator.HasColumn(crudLogModel, "Sync") {
		if err := crudMigrator.AddColumn(crudLogModel, "Sync"); err != nil {
			return fmt.Errorf("add sync column to %s: %w", crudLogTable, err)
		}
	}

	// 历史回填：从 crud_log.table JSON 中提取 comment
	var logs []struct {
		ID    int32  `gorm:"column:id"`
		Table string `gorm:"column:table"`
	}
	if err := db.Table(crudLogTable).Where("comment = ?", "").Find(&logs).Error; err != nil {
		return fmt.Errorf("query crud_log for backfill: %w", err)
	}
	for _, log := range logs {
		var data struct {
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal([]byte(log.Table), &data); err != nil {
			continue // 解析失败跳过单条
		}
		if data.Comment == "" {
			continue
		}
		if err := db.Table(crudLogTable).Where("id = ?", log.ID).Update("comment", data.Comment).Error; err != nil {
			return fmt.Errorf("backfill crud_log id=%d: %w", log.ID, err)
		}
	}
	return nil
}
