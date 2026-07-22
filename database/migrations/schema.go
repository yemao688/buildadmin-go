package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/official"

	"gorm.io/gorm"
)

const installDataVersion = official.InstallDataVersion
const installDataName = official.InstallDataName

type InstallRecoveryState = official.InstallRecoveryState
type migrationRecord = core.MigrationRecord
type migrationColumn = core.MigrationColumn

const (
	InstallFresh         = official.InstallFresh
	InstallInterrupted   = official.InstallInterrupted
	InstallStrictUpgrade = official.InstallStrictUpgrade
)

func ValidatePrefix(config *conf.Configuration) error { return core.ValidatePrefix(config) }
func tableName(config *conf.Configuration, logicalName string) string {
	return core.TableName(config, logicalName)
}
func quoteIdentifier(value string) string { return core.QuoteIdentifier(value) }
func legacyTableExists(db *gorm.DB, name string) (bool, error) {
	return core.LegacyTableExists(db, name)
}
func legacyColumnExists(db *gorm.DB, table, column string) (bool, error) {
	return core.LegacyColumnExists(db, table, column)
}
func tableExists(db *gorm.DB, name string) bool { return core.TableExists(db, name) }
func columnExists(db *gorm.DB, table, column string) bool {
	return core.ColumnExists(db, table, column)
}
func indexExists(db *gorm.DB, table, index string) bool {
	return core.IndexExists(db, table, index)
}
func indexFirstColumn(db *gorm.DB, table, index string) (string, error) {
	return core.IndexFirstColumn(db, table, index)
}
func migrationColumnInfo(db *gorm.DB, table, column string) (migrationColumn, bool, error) {
	return core.MigrationColumnInfo(db, table, column)
}
func migrationIndexInfo(db *gorm.DB, table, index string) (bool, string, error) {
	return core.MigrationIndexInfo(db, table, index)
}
func MarkSeedPending(db *gorm.DB, config *conf.Configuration) error {
	return official.MarkSeedPending(db, config)
}
func SeedPending(db *gorm.DB, config *conf.Configuration) (bool, error) {
	return official.SeedPending(db, config)
}
func MarkSeedCompleted(db *gorm.DB, config *conf.Configuration) error {
	return official.MarkSeedCompleted(db, config)
}
func DecideInstallRecovery(db *gorm.DB, config *conf.Configuration) (InstallRecoveryState, error) {
	return official.DecideInstallRecovery(db, config)
}
func IsFreshDatabase(db *gorm.DB, config *conf.Configuration) (bool, error) {
	return official.IsFreshDatabase(db, config)
}
func SeedCurrentData(db *gorm.DB, config *conf.Configuration) (bool, error) {
	return official.SeedCurrentData(db, config)
}
func ValidateCurrentSchema(db *gorm.DB, config *conf.Configuration) error {
	return official.ValidateCurrentSchema(db, config)
}
