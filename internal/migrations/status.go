package migrations

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/business"

	"gorm.io/gorm"
)

// TrackStatus 描述单条迁移轨道的注册/已应用数量、未应用名单与未完成记录。
type TrackStatus struct {
	Track      string // official | framework | business
	Registered int
	Applied    int
	Pending    int
	Incomplete int
	// PendingNames 是已注册但未在台账标记完成的名字，保持注册顺序。
	PendingNames []string
}

// StatusReport 汇总安装状态与三条迁移轨道的进度。
type StatusReport struct {
	State  InstallRecoveryState
	Tracks []TrackStatus
}

// Status 只读检查安装状态与三条迁移轨道的注册/已应用/未完成数量。
func Status(db *gorm.DB, config *conf.Configuration) (StatusReport, error) {
	state, err := DecideInstallRecovery(db, config)
	if err != nil {
		return StatusReport{}, err
	}
	report := StatusReport{State: state}
	// 未进入严格升级态时台账可能尚不存在，只回报安装状态。
	if state != InstallStrictUpgrade {
		return report, nil
	}

	official, frameworks := OfficialMigrations(), FrameworkMigrations()
	businessMigrations, err := business.Migrations()
	if err != nil {
		return report, err
	}

	tracks := []struct {
		name   string
		ledger string
		names  []string
	}{
		{name: "official", ledger: "migrations", names: officialMigrationNames(official)},
		{name: "framework", ledger: "migrations_framework", names: frameworkMigrationNames(frameworks)},
		{name: "business", ledger: "migrations_business", names: businessMigrationNames(businessMigrations)},
	}
	for _, track := range tracks {
		status, err := readTrackStatus(db, config, track.name, track.ledger, track.names)
		if err != nil {
			return report, err
		}
		report.Tracks = append(report.Tracks, status)
	}
	return report, nil
}

func readTrackStatus(db *gorm.DB, config *conf.Configuration, track, logicalName string, names []string) (TrackStatus, error) {
	ledger := tableName(config, logicalName)
	// 框架台账可能早于本次升级尚未建立：按全部待应用处理，避免首次升级直接失败。
	if !tableExists(db, ledger) {
		return summarizeTrack(track, names, nil, 0), nil
	}

	var completed []string
	if err := db.Table(ledger).Where("end_time IS NOT NULL").Pluck("migration_name", &completed).Error; err != nil {
		return TrackStatus{}, err
	}
	var incomplete int64
	if err := db.Table(ledger).Where("end_time IS NULL").Count(&incomplete).Error; err != nil {
		return TrackStatus{}, err
	}
	return summarizeTrack(track, names, completed, int(incomplete)), nil
}

// summarizeTrack 按注册名单与台账已完成名单计算单轨进度；台账缺失时传入 nil（全部待应用）。
func summarizeTrack(track string, registered, completed []string, incomplete int) TrackStatus {
	status := TrackStatus{Track: track, Registered: len(registered), Incomplete: incomplete}
	done := make(map[string]bool, len(completed))
	for _, name := range completed {
		done[name] = true
	}
	for _, name := range registered {
		if done[name] {
			status.Applied++
			continue
		}
		status.PendingNames = append(status.PendingNames, name)
	}
	status.Pending = status.Registered - status.Applied
	if status.Pending < 0 {
		status.Pending = 0
	}
	return status
}

func officialMigrationNames(list []OfficialMigration) []string {
	names := make([]string, 0, len(list))
	for _, migration := range list {
		names = append(names, migration.Key.Name)
	}
	return names
}

func frameworkMigrationNames(list []FrameworkMigration) []string {
	names := make([]string, 0, len(list))
	for _, migration := range list {
		names = append(names, migration.MigrationName)
	}
	return names
}

func businessMigrationNames(list []business.Migration) []string {
	names := make([]string, 0, len(list))
	for _, migration := range list {
		names = append(names, migration.MigrationName)
	}
	return names
}
