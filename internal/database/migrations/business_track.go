package migrations

import (
	"errors"
	"fmt"
	"time"

	"go-build-admin/internal/conf"
	"go-build-admin/internal/database/migrations/business"
	"go-build-admin/internal/database/migrations/internal/core"
	"gorm.io/gorm"
)

const businessLedgerName = "migrations_business"

func BusinessMigrations() ([]business.Migration, error) {
	return business.Migrations()
}

func BootstrapBusinessLedger(db *gorm.DB, config *conf.Configuration) error {
	return core.BootstrapTrackedLedger(db, config, businessLedgerName)
}

func ValidateBusinessLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateTrackedLedgerSchema(db, config, businessLedgerName)
}

func RunBusinessMigrations(db *gorm.DB, config *conf.Configuration, list []business.Migration) (int, error) {
	tracked := make([]core.TrackedMigration, 0, len(list))
	for _, migration := range list {
		tracked = append(tracked, core.TrackedMigration{Version: migration.Version, MigrationName: migration.MigrationName, Up: migration.Up, Down: migration.Down, VerifyBaseline: migration.VerifyBaseline, VerifySchema: migration.VerifySchema, VerifyUpgradeData: migration.VerifyUpgradeData})
	}
	return core.RunTrackedMigrations(db, config, businessLedgerName, tracked, core.TrackedRunnerOptions{TrackName: "business"})
}

type RollbackOptions struct {
	Steps         uint64
	ToBreakpoint  bool
	TargetVersion *uint64
}

type RollbackReport = core.RollbackReport
type RollbackEntry = core.RollbackEntry

func RollbackBusinessMigrations(db *gorm.DB, config *conf.Configuration, list []business.Migration, options RollbackOptions) (RollbackReport, error) {
	target := options.TargetVersion
	if options.ToBreakpoint {
		breakpoint, err := ReadBusinessBreakpoint(db, config)
		if err != nil {
			return RollbackReport{}, err
		}
		if breakpoint == nil {
			return RollbackReport{}, errors.New("no business migration breakpoint is set")
		}
		target = &breakpoint.Version
	}
	tracked := make([]core.TrackedMigration, 0, len(list))
	for _, migration := range list {
		tracked = append(tracked, core.TrackedMigration{Version: migration.Version, MigrationName: migration.MigrationName, Up: migration.Up, Down: migration.Down})
	}
	return core.RollbackTrackedMigrations(db, config, businessLedgerName, tracked, core.RollbackOptions{Steps: options.Steps, TargetVersion: target, TrackName: "business"})
}

type BusinessBreakpoint struct {
	Version  uint64
	Sequence uint64
	SetTime  time.Time
}

func SetBusinessBreakpoint(db *gorm.DB, config *conf.Configuration, version uint64) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	table := core.QuoteIdentifier(core.TableName(config, businessLedgerName))
	if err := db.Exec("UPDATE " + table + " SET breakpoint = 0").Error; err != nil {
		return err
	}
	result := db.Exec("UPDATE "+table+" SET breakpoint = 1 WHERE version = ?", version)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("business migration version %d does not exist", version)
	}
	return nil
}

func ReadBusinessBreakpoint(db *gorm.DB, config *conf.Configuration) (*BusinessBreakpoint, error) {
	if err := core.ValidatePrefix(config); err != nil {
		return nil, err
	}
	var row struct {
		Version   uint64
		StartTime time.Time `gorm:"column:start_time"`
	}
	result := db.Raw("SELECT version,start_time FROM " + core.QuoteIdentifier(core.TableName(config, businessLedgerName)) + " WHERE breakpoint = 1 LIMIT 1").Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &BusinessBreakpoint{Version: row.Version, Sequence: row.Version, SetTime: row.StartTime}, nil
}

func ClearBusinessBreakpoint(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("UPDATE " + core.QuoteIdentifier(core.TableName(config, businessLedgerName)) + " SET breakpoint = 0").Error
}
