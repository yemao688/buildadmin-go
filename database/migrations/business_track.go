package migrations

import (
	"errors"
	"fmt"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/business"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

const businessLedgerName = "business_migrations"

func BusinessMigrations() ([]business.Migration, error) {
	return business.Migrations()
}

func BootstrapBusinessLedger(db *gorm.DB, config *conf.Configuration) error {
	if err := core.BootstrapTrackedLedger(db, config, businessLedgerName, core.TrackedLedgerOptions{IncludeBatch: true}); err != nil {
		return err
	}
	return BootstrapBusinessBreakpoint(db, config)
}

func ValidateBusinessLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateTrackedLedgerSchema(db, config, businessLedgerName, core.TrackedLedgerOptions{IncludeBatch: true})
}

func RunBusinessMigrations(db *gorm.DB, config *conf.Configuration, list []business.Migration) (int, error) {
	tracked := make([]core.TrackedMigration, 0, len(list))
	for _, migration := range list {
		tracked = append(tracked, core.TrackedMigration{Sequence: migration.Sequence, ID: migration.ID, Revision: migration.Revision, Up: migration.Up, Down: migration.Down, VerifyBaseline: migration.VerifyBaseline, VerifySchema: migration.VerifySchema, VerifyUpgradeData: migration.VerifyUpgradeData})
	}
	return core.RunTrackedMigrations(db, config, businessLedgerName, tracked, core.TrackedRunnerOptions{TrackName: "business", IncludeBatch: true})
}

type RollbackOptions struct {
	Steps          uint64
	ToBreakpoint   bool
	TargetSequence *uint64
}

type RollbackReport = core.RollbackReport
type RollbackEntry = core.RollbackEntry

func RollbackBusinessMigrations(db *gorm.DB, config *conf.Configuration, list []business.Migration, options RollbackOptions) (RollbackReport, error) {
	target := options.TargetSequence
	if options.ToBreakpoint {
		breakpoint, err := ReadBusinessBreakpoint(db, config)
		if err != nil {
			return RollbackReport{}, err
		}
		if breakpoint == nil {
			return RollbackReport{}, errors.New("no business migration breakpoint is set")
		}
		target = &breakpoint.Sequence
	}
	tracked := make([]core.TrackedMigration, 0, len(list))
	for _, migration := range list {
		tracked = append(tracked, core.TrackedMigration{Sequence: migration.Sequence, ID: migration.ID, Revision: migration.Revision, Up: migration.Up, Down: migration.Down})
	}
	return core.RollbackTrackedMigrations(db, config, businessLedgerName, tracked, core.RollbackOptions{Steps: options.Steps, TargetSequence: target, TrackName: "business"})
}

const businessBreakpointTableName = "business_breakpoints"

type BusinessBreakpoint struct {
	ID       uint8     `gorm:"column:id"`
	Sequence uint64    `gorm:"column:sequence"`
	SetTime  time.Time `gorm:"column:set_time"`
}

func BootstrapBusinessBreakpoint(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(core.TableName(config, businessBreakpointTableName)) + " (" +
		"`id` TINYINT UNSIGNED NOT NULL, `sequence` BIGINT UNSIGNED NOT NULL, `set_time` TIMESTAMP(6) NOT NULL, " +
		"PRIMARY KEY (`id`)) ENGINE=InnoDB").Error
}

func SetBusinessBreakpoint(db *gorm.DB, config *conf.Configuration, sequence uint64) error {
	if err := BootstrapBusinessBreakpoint(db, config); err != nil {
		return err
	}
	table := core.QuoteIdentifier(core.TableName(config, businessBreakpointTableName))
	return db.Exec("INSERT INTO "+table+" (id,sequence,set_time) VALUES (1,?,?) ON DUPLICATE KEY UPDATE sequence=VALUES(sequence), set_time=VALUES(set_time)", sequence, time.Now()).Error
}

func ReadBusinessBreakpoint(db *gorm.DB, config *conf.Configuration) (*BusinessBreakpoint, error) {
	if err := BootstrapBusinessBreakpoint(db, config); err != nil {
		return nil, err
	}
	var breakpoint BusinessBreakpoint
	result := db.Table(core.TableName(config, businessBreakpointTableName)).Where("id = 1").First(&breakpoint)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &breakpoint, nil
}

func ClearBusinessBreakpoint(db *gorm.DB, config *conf.Configuration) error {
	if err := BootstrapBusinessBreakpoint(db, config); err != nil {
		return err
	}
	return db.Exec("DELETE FROM " + core.QuoteIdentifier(core.TableName(config, businessBreakpointTableName)) + " WHERE id = 1").Error
}

func ValidateBusinessBreakpointSchema(db *gorm.DB, config *conf.Configuration) error {
	if err := BootstrapBusinessBreakpoint(db, config); err != nil {
		return err
	}
	var engine string
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", core.TableName(config, businessBreakpointTableName)).Scan(&engine).Error; err != nil {
		return err
	}
	if engine != "InnoDB" {
		return fmt.Errorf("%s schema mismatch: engine=%q", businessBreakpointTableName, engine)
	}
	return nil
}
