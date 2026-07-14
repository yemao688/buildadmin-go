package migrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"
	"gorm.io/gorm"
)

type migrationRecord struct {
	Version       int64      `gorm:"column:version"`
	MigrationName string     `gorm:"column:migration_name"`
	StartTime     time.Time  `gorm:"column:start_time"`
	EndTime       *time.Time `gorm:"column:end_time"`
	Breakpoint    bool       `gorm:"column:breakpoint"`
}

type MigrationFn func(*gorm.DB, *conf.Configuration) error

type VersionMigration struct {
	Version int64
	Name    string
	Up      MigrationFn
}

// tableName 获取带前缀的表名
func tableName(config *conf.Configuration, logicalName string) string {
	return config.Database.Prefix + logicalName
}

// ─── Version205: 配置快捷入口从 /admin/ 路径迁移到路由 name ───
func version205(db *gorm.DB, config *conf.Configuration) error {
	var cfgs []model.Config
	result := db.Table(tableName(config, "config")).Where("name = ?", "config_quick_entrance").Find(&cfgs)
	if result.Error != nil {
		return result.Error
	}
	if len(cfgs) == 0 {
		return nil
	}
	cfg := cfgs[0]
	if strings.TrimSpace(cfg.Value) == "" {
		return nil
	}

	// 使用 map[string]any 保留所有字段，避免重编码时丢失未知属性
	var entries []map[string]any
	if err := json.Unmarshal([]byte(cfg.Value), &entries); err != nil {
		return fmt.Errorf("parse config_quick_entrance: %w", err)
	}

	changed := false
	for i := range entries {
		val, ok := entries[i]["value"].(string)
		if !ok || !strings.HasPrefix(val, "/admin/") {
			continue
		}
		path := strings.TrimPrefix(val, "/admin/")
		var rules []model.AdminRule
		result := db.Table(tableName(config, "admin_rule")).Where("path = ?", path).Find(&rules)
		if result.Error != nil {
			return result.Error
		}
		if len(rules) == 0 {
			continue
		}
		rule := rules[0]
		if val != rule.Name {
			entries[i]["value"] = rule.Name
			changed = true
		}
	}
	if !changed {
		return nil
	}
	value, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return db.Table(tableName(config, "config")).Where("id = ?", cfg.ID).Update("value", string(value)).Error
}

// ─── Version206: 5 张表新增 connection 字段（跳过 PHP 专属 backend_entrance） ───
func version206(db *gorm.DB, config *conf.Configuration) error {
	tables := []struct {
		name  string
		model any
	}{
		{"crud_log", &model.CrudLog{}},
		{"security_data_recycle", &model.SecurityDataRecycle{}},
		{"security_data_recycle_log", &model.SecurityDataRecycleLog{}},
		{"security_sensitive_data", &model.SecuritySensitiveData{}},
		{"security_sensitive_data_log", &model.SecuritySensitiveDataLog{}},
	}
	for _, item := range tables {
		fullTable := tableName(config, item.name)
		migrator := db.Table(fullTable).Migrator()
		if !migrator.HasTable(item.model) {
			continue
		}
		if migrator.HasColumn(item.model, "Connection") {
			continue
		}
		if err := migrator.AddColumn(item.model, "Connection"); err != nil {
			return fmt.Errorf("add connection column to %s: %w", fullTable, err)
		}
	}
	return nil
}

// ─── Version222: 列类型扩容 + crud_log 新增字段 + 历史回填 ───
// 跳过：status 类型迁移（0/1 → enable/disable）、7 张表 status 值映射
func version222(db *gorm.DB, config *conf.Configuration) error {
	// 列类型扩容
	type alterSpec struct {
		table string
		model any
		field string
	}
	alterSpecs := []alterSpec{
		{"attachment", &model.Attachment{}, "Name"},
		{"admin", &model.Admin{}, "Password"},
		{"user", &model.User{}, "Password"},
	}
	for _, spec := range alterSpecs {
		fullTable := tableName(config, spec.table)
		if err := db.Table(fullTable).Migrator().AlterColumn(spec.model, spec.field); err != nil {
			return fmt.Errorf("alter %s.%s: %w", fullTable, spec.field, err)
		}
	}

	// crud_log 新增 comment 和 sync（独立检查，幂等）
	crudLogTable := tableName(config, "crud_log")
	crudLogModel := &model.CrudLog{}
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

// ─── 迁移注册 ───

var allMigrations = []VersionMigration{
	{Version: 20231112093414, Name: "Version205", Up: version205},
	{Version: 20231229043002, Name: "Version206", Up: version206},
	{Version: 20250412134127, Name: "Version222", Up: version222},
}

// validateMigrations 验证迁移列表的版本号严格递增、名称非空
func validateMigrations() error {
	var previous int64
	for i, m := range allMigrations {
		if m.Version <= previous {
			return fmt.Errorf("migration versions must be strictly increasing: %d", m.Version)
		}
		if i > 0 && m.Version == allMigrations[i-1].Version {
			return fmt.Errorf("duplicate migration version: %d", m.Version)
		}
		if strings.TrimSpace(m.Name) == "" || m.Up == nil {
			return fmt.Errorf("invalid migration at version %d", m.Version)
		}
		previous = m.Version
	}
	return nil
}

// RunVersionMigrations 执行尚未完成的版本迁移
// 每个迁移必须幂等；成功后才写入 migration record
func RunVersionMigrations(db *gorm.DB, config *conf.Configuration) (int, error) {
	if err := validateMigrations(); err != nil {
		return 0, err
	}
	migrationsTable := tableName(config, "migrations")
	count := 0

	for _, migration := range allMigrations {
		// 查询该版本是否存在已完成记录
		var record migrationRecord
		recordDB := db.Session(&gorm.Session{})
		result := recordDB.Table(migrationsTable).Where("version = ?", migration.Version).Limit(1).Find(&record)
		if result.Error != nil {
			return count, fmt.Errorf("query migration version %d: %w", migration.Version, result.Error)
		}

		recordExists := result.RowsAffected > 0
		if recordExists && record.EndTime != nil {
			// 已完成：验证名称一致（collision 检测）
			if record.MigrationName != migration.Name {
				return count, fmt.Errorf("migration version %d: name collision (db=%s, code=%s)",
					migration.Version, record.MigrationName, migration.Name)
			}
			continue // 跳过已完成的迁移
		}

		// 记录开始时间
		start := time.Now()

		// 执行迁移
		migrationDB := db.Session(&gorm.Session{})
		if err := migration.Up(migrationDB, config); err != nil {
			return count, fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}

		end := time.Now()

		// 成功后才写入/更新 migration record
		var writeErr error
		if recordExists {
			writeErr = db.Session(&gorm.Session{}).Table(migrationsTable).Where("version = ?", migration.Version).Updates(map[string]any{
				"migration_name": migration.Name,
				"start_time":     start,
				"end_time":       end,
			}).Error
		} else {
			writeErr = db.Session(&gorm.Session{}).Exec(
				"INSERT INTO `"+migrationsTable+"` (`version`, `migration_name`, `start_time`, `end_time`, `breakpoint`) VALUES (?, ?, ?, ?, ?)",
				migration.Version, migration.Name, start, end, false,
			).Error
		}
		if writeErr != nil {
			return count, fmt.Errorf("write migration record for %s: %w", migration.Name, writeErr)
		}
		count++
	}
	return count, nil
}
