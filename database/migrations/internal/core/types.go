package core

import (
	"fmt"
	"strings"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type MigrationFn func(*gorm.DB, *conf.Configuration) error

type OfficialKey struct {
	Version int64
	Name    string
}

type OfficialMigration struct {
	Key    OfficialKey
	Source string
	Up     MigrationFn
}

type FrameworkMigration struct {
	Sequence uint64
	ID       string
	Revision uint64
	// RequiresOfficial is retained in the core track contract because the
	// framework runner enforces official completion before executing framework Up.
	RequiresOfficial  []OfficialKey
	Up                MigrationFn
	VerifyBaseline    MigrationFn
	VerifySchema      func(*gorm.DB, *conf.Configuration) error
	VerifyUpgradeData func(*gorm.DB, *conf.Configuration) error
}

type FrameworkMigrationRecord struct {
	Sequence    uint64     `gorm:"column:sequence"`
	MigrationID string     `gorm:"column:migration_id"`
	Revision    uint64     `gorm:"column:revision"`
	StartTime   time.Time  `gorm:"column:start_time"`
	EndTime     *time.Time `gorm:"column:end_time"`
}

func (FrameworkMigrationRecord) TableName() string { return "framework_migrations" }

func ValidateOfficialMigrations(list []OfficialMigration) error {
	var previous int64
	seen := map[int64]string{}
	for i, m := range list {
		if m.Key.Version <= previous || m.Key.Version == 0 || strings.TrimSpace(m.Key.Name) == "" || strings.TrimSpace(m.Source) == "" || m.Up == nil {
			return fmt.Errorf("invalid official migration at index %d", i)
		}
		if old, ok := seen[m.Key.Version]; ok && old != m.Key.Name {
			return fmt.Errorf("official version %d name collision", m.Key.Version)
		}
		seen[m.Key.Version] = m.Key.Name
		previous = m.Key.Version
	}
	return nil
}

func ValidateFrameworkMigrations(list []FrameworkMigration, official []OfficialMigration) error {
	if err := ValidateOfficialMigrations(official); err != nil {
		return err
	}
	officialKeys := map[OfficialKey]bool{}
	for _, m := range official {
		officialKeys[m.Key] = true
	}
	seenID, seenSeq := map[string]bool{}, map[uint64]bool{}
	var previousSequence uint64
	for i, m := range list {
		if m.Sequence == 0 || m.Sequence <= previousSequence || strings.TrimSpace(m.ID) == "" || m.Revision == 0 || m.Up == nil || seenID[m.ID] || seenSeq[m.Sequence] {
			return fmt.Errorf("invalid framework migration at index %d", i)
		}
		for _, key := range m.RequiresOfficial {
			if !officialKeys[key] {
				return fmt.Errorf("framework migration %s requires unknown official migration %d/%s", m.ID, key.Version, key.Name)
			}
		}
		seenID[m.ID], seenSeq[m.Sequence] = true, true
		previousSequence = m.Sequence
	}
	return nil
}
