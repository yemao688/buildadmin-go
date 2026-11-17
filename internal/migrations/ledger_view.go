package migrations

import (
	"time"

	"buildadmin-go/internal/conf"

	"gorm.io/gorm"
)

// LedgerTracks 是迁移台账的展示顺序：official、framework、business。
var LedgerTracks = []string{"official", "framework", "business"}

// LedgerEntry 是迁移台账的一行；StartTime 为 nil 表示台账未记录开始时间。
type LedgerEntry struct {
	Track         string
	Version       uint64
	MigrationName string
	StartTime     *time.Time
	EndTime       *time.Time
	Breakpoint    bool
}

type ledgerRow struct {
	Version       uint64     `gorm:"column:version"`
	MigrationName string     `gorm:"column:migration_name"`
	StartTime     *time.Time `gorm:"column:start_time"`
	EndTime       *time.Time `gorm:"column:end_time"`
	Breakpoint    bool       `gorm:"column:breakpoint"`
}

// LedgerEntries 按 LedgerTracks 顺序只读读取三轨台账；不存在的台账跳过。
func LedgerEntries(db *gorm.DB, config *conf.Configuration) ([]LedgerEntry, error) {
	entries := make([]LedgerEntry, 0, 16)
	for _, track := range LedgerTracks {
		table := tableName(config, ledgerTableName(track))
		if !tableExists(db, table) {
			continue
		}
		var rows []ledgerRow
		if err := db.Table(table).
			Select("version,migration_name,start_time,end_time,breakpoint").
			Order("version").
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			entries = append(entries, LedgerEntry{
				Track:         track,
				Version:       row.Version,
				MigrationName: row.MigrationName,
				StartTime:     row.StartTime,
				EndTime:       row.EndTime,
				Breakpoint:    row.Breakpoint,
			})
		}
	}
	return entries, nil
}

func ledgerTableName(track string) string {
	switch track {
	case "official":
		return "migrations"
	case "framework":
		return "migrations_framework"
	default:
		return businessLedgerName
	}
}
