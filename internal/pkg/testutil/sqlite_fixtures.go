package testutil

import (
	"gorm.io/gorm"
)

// SQLite fixture DDL helpers.
//
// The shared entity layer (internal/model) and per-table owner entities
// carry MySQL-dialect gorm type tags (int unsigned, enum, ...), which
// sqlite AutoMigrate cannot parse. Tests that used to AutoMigrate these
// entities now create their fixture tables with sqlite-native DDL
// (column names/nullability/primary keys keep the test semantics; types
// are the sqlite-accepted INTEGER/TEXT/REAL forms). These helpers
// centralize the column shapes so the affected packages do not each
// duplicate them. Callers resolve table names through their own
// NamingStrategy (prefix/singular) and pass them in explicitly.

// createSQLiteTable runs CREATE TABLE IF NOT EXISTS: some tests share one
// in-memory database through a "cache=shared" DSN, so the helper must be
// idempotent just like AutoMigrate was.
func createSQLiteTable(db *gorm.DB, table, columns string) error {
	return db.Exec("CREATE TABLE IF NOT EXISTS `" + table + "` (" + columns + ")").Error
}

// CreateSQLiteUserTables creates the fixture tables for model.User and its
// belongsTo target simple.Admin (e.g. "users" + "admins" under the default
// naming strategy, or "ba_user" + "ba_admin" with a ba_ prefix).
func CreateSQLiteUserTables(db *gorm.DB, userTable, adminTable string) error {
	if err := createSQLiteTable(db, userTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL DEFAULT 0,
		username TEXT NOT NULL DEFAULT '',
		nickname TEXT NOT NULL DEFAULT '',
		avatar TEXT NOT NULL DEFAULT '',
		email TEXT NOT NULL DEFAULT '',
		mobile TEXT NOT NULL DEFAULT '',
		password TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'enable',
		money REAL NOT NULL DEFAULT 0,
		last_login_time INTEGER,
		last_login_ip TEXT NOT NULL DEFAULT '',
		login_failure INTEGER NOT NULL DEFAULT 0,
		join_ip TEXT NOT NULL DEFAULT '',
		join_time INTEGER,
		create_time INTEGER,
		update_time INTEGER
	`); err != nil {
		return err
	}
	return createSQLiteTable(db, adminTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		nickname TEXT NOT NULL
	`)
}

// CreateSQLiteAdminLogTable creates the fixture table for model.AdminLog
// (e.g. "admin_logs" pluralized, "ba_admin_log" prefixed, or "admin_log"
// singular).
func CreateSQLiteAdminLogTable(db *gorm.DB, table string) error {
	return createSQLiteTable(db, table, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL DEFAULT 0,
		username TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		data TEXT,
		ip TEXT NOT NULL DEFAULT '',
		useragent TEXT NOT NULL DEFAULT '',
		create_time INTEGER
	`)
}

// CreateSQLiteAdminRuleTables creates the fixture tables for model.AdminRule,
// model.AdminGroup and model.AdminGroupAccess (e.g. "admin_rule" /
// "admin_group" / "admin_group_access" singular, or the ba_ prefixed forms).
func CreateSQLiteAdminRuleTables(db *gorm.DB, ruleTable, groupTable, accessTable string) error {
	if err := createSQLiteTable(db, ruleTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pid INTEGER NOT NULL DEFAULT 0,
		"type" TEXT NOT NULL DEFAULT 'menu',
		title TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		path TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		menu_type TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		component TEXT NOT NULL DEFAULT '',
		keepalive INTEGER NOT NULL DEFAULT 0,
		extend TEXT NOT NULL DEFAULT 'none',
		remark TEXT NOT NULL DEFAULT '',
		weigh INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT '1',
		update_time INTEGER,
		create_time INTEGER
	`); err != nil {
		return err
	}
	if err := createSQLiteTable(db, groupTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		pid INTEGER NOT NULL DEFAULT 0,
		name TEXT NOT NULL DEFAULT '',
		rules TEXT,
		status TEXT NOT NULL DEFAULT '1',
		update_time INTEGER,
		create_time INTEGER
	`); err != nil {
		return err
	}
	return createSQLiteTable(db, accessTable, `
		uid INTEGER NOT NULL,
		group_id INTEGER NOT NULL
	`)
}

// CreateSQLiteConfigTable creates the fixture table for siteconfig.Config
// (e.g. "configs" pluralized under the default strategy, or "ba_config"
// with a ba_ prefix and singular tables).
func CreateSQLiteConfigTable(db *gorm.DB, table string) error {
	return createSQLiteTable(db, table, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		"group" TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		tip TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL DEFAULT '',
		value TEXT,
		content TEXT,
		rule TEXT NOT NULL DEFAULT '',
		extend TEXT NOT NULL DEFAULT '',
		allow_del INTEGER NOT NULL DEFAULT 0,
		weigh INTEGER NOT NULL DEFAULT 0
	`)
}

// CreateSQLiteTokenTable creates the fixture table for token.Token
// (pluralized to "tokens" under the default naming strategy).
func CreateSQLiteTokenTable(db *gorm.DB, table string) error {
	return createSQLiteTable(db, table, `
		token TEXT NOT NULL DEFAULT '' PRIMARY KEY,
		type TEXT NOT NULL DEFAULT '',
		user_id INTEGER NOT NULL DEFAULT 0,
		create_time INTEGER,
		expire_time INTEGER
	`)
}

// CreateSQLiteAttachmentTables creates the fixture tables for
// upload.Attachment and its belongsTo targets upload.AttachmentAdmin /
// upload.AttachmentUser (e.g. "ba_attachment" + "ba_admin" + "ba_user"
// with a ba_ prefix).
func CreateSQLiteAttachmentTables(db *gorm.DB, attachmentTable, adminTable, userTable string) error {
	if err := createSQLiteTable(db, attachmentTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id INTEGER NOT NULL DEFAULT 0,
		user_id INTEGER NOT NULL DEFAULT 0,
		topic TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		width INTEGER NOT NULL DEFAULT 0,
		height INTEGER NOT NULL DEFAULT 0,
		name TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		mimetype TEXT NOT NULL DEFAULT '',
		quote INTEGER NOT NULL DEFAULT 0,
		storage TEXT NOT NULL DEFAULT '',
		sha1 TEXT NOT NULL DEFAULT '',
		create_time INTEGER,
		last_upload_time INTEGER
	`); err != nil {
		return err
	}
	if err := createSQLiteTable(db, adminTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		nickname TEXT NOT NULL
	`); err != nil {
		return err
	}
	return createSQLiteTable(db, userTable, `
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		nickname TEXT NOT NULL
	`)
}
