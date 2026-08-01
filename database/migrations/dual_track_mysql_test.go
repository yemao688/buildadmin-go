package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func TestDualTrackMySQLLedgerAndLock(t *testing.T) {
	db := getDB(t)
	config := &conf.Configuration{}
	config.Database.Prefix = "phase1_"
	if err := db.Exec("DROP TABLE IF EXISTS `phase1_migrations_framework`").Error; err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP TABLE IF EXISTS `phase1_migrations_framework`")
	if err := BootstrapFrameworkLedger(db, config); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFrameworkLedgerSchema(db, config); err != nil {
		t.Fatal(err)
	}
	framework := FrameworkMigration{Version: 1, MigrationName: "retryable", Up: func(_ *gorm.DB, _ *conf.Configuration) error { return nil }}
	if err := InsertPendingFrameworkMigration(db, config, framework); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, config, nil, []FrameworkMigration{framework}); err != nil {
		t.Fatal(err)
	}
	var locks sync.WaitGroup
	locks.Add(2)
	entered := make(chan struct{})
	var enteredOnce sync.Once
	var active int32
	var overlap atomic.Bool
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer locks.Done()
			results <- WithMigrationLock(db, "phase1-test-lock", 2*time.Second, func(_ *gorm.DB) error {
				enteredOnce.Do(func() { close(entered) })
				if atomic.AddInt32(&active, 1) > 1 {
					overlap.Store(true)
				}
				time.Sleep(150 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			})
		}()
	}
	locks.Wait()
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if overlap.Load() {
		t.Fatal("lock callbacks overlapped")
	}
	var connectionID, usedLock sql.NullInt64
	if err := WithMigrationLock(db, "phase1-identity-lock", time.Second, func(pinned *gorm.DB) error {
		return pinned.Raw("SELECT CONNECTION_ID(), IS_USED_LOCK(?)", "phase1-identity-lock").Row().Scan(&connectionID, &usedLock)
	}); err != nil {
		t.Fatal(err)
	}
	if !connectionID.Valid || !usedLock.Valid || connectionID.Int64 != usedLock.Int64 {
		t.Fatalf("lock connection identity=%v used=%v", connectionID, usedLock)
	}
	callbackError := errors.New("callback error")
	if err := WithMigrationLock(db, "phase1-error-lock", time.Second, func(_ *gorm.DB) error { return callbackError }); !errors.Is(err, callbackError) {
		t.Fatalf("callback error=%v", err)
	}
	if err := WithMigrationLock(db, "phase1-error-lock", time.Second, func(_ *gorm.DB) error { return nil }); err != nil {
		t.Fatal("error path did not release:", err)
	}
	held := make(chan struct{})
	releaseHeld := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- WithMigrationLock(db, "phase1-timeout-lock", 2*time.Second, func(_ *gorm.DB) error { close(held); <-releaseHeld; return nil })
	}()
	<-held
	var timedOutCallback atomic.Bool
	timeoutErr := WithMigrationLock(db, "phase1-timeout-lock", 100*time.Millisecond, func(_ *gorm.DB) error { timedOutCallback.Store(true); return nil })
	close(releaseHeld)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if timeoutErr == nil || timedOutCallback.Load() {
		t.Fatalf("timeout callback ran or returned nil: %v", timeoutErr)
	}
	panicLock := "phase1-panic-lock-" + time.Now().Format("150405.000000")
	var panicErr error
	var panicCallback bool
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				panicErr = errors.New("panic was not propagated")
			}
		}()
		result := WithMigrationLock(db, panicLock, time.Second, func(_ *gorm.DB) error { panicCallback = true; panic("boom") })
		if result != nil {
			panicErr = result
		}
	}()
	if panicErr != nil {
		t.Fatal(panicErr)
	}
	if !panicCallback {
		t.Fatal("panic callback did not execute")
	}
	if err := WithMigrationLock(db, panicLock, time.Second, func(_ *gorm.DB) error { return nil }); err != nil {
		t.Fatal("lock was not released:", err)
	}
}

func TestFrameworkFinalSeedPinnedConnection(t *testing.T) {
	db := getDB(t)
	config := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("pinned_%d_", time.Now().UnixNano())}}
	q := func(logical string) string { return quoteIdentifier(tableName(config, logical)) }
	for _, logical := range []string{"admin", "user", "attachment", "user_money_log", "admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log", "admin_closure", "admin_rule", "config", "country_language", "country_language_content", "country_currency"} {
		db.Exec("DROP TABLE IF EXISTS " + q(logical))
		table := q(logical)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + table) })
	}
	for _, ddl := range []string{
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, parent_id INT NULL)",
		"CREATE TABLE " + q("attachment") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("user") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, status VARCHAR(30) NOT NULL DEFAULT 'enable', money INT UNSIGNED NULL)",
		"CREATE TABLE " + q("user_money_log") + " (id INT PRIMARY KEY, user_id INT, admin_id INT UNSIGNED NOT NULL DEFAULT 0, money INT UNSIGNED, `before` INT UNSIGNED, `after` INT UNSIGNED)",
		"CREATE TABLE " + q("admin_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_data_recycle_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, recycle_id INT UNSIGNED NOT NULL DEFAULT 0, data TEXT, data_table VARCHAR(100) NOT NULL DEFAULT '', primary_key VARCHAR(50) NOT NULL DEFAULT '', is_restore TINYINT UNSIGNED NOT NULL DEFAULT 0, ip VARCHAR(50) NOT NULL DEFAULT '', useragent VARCHAR(255) NOT NULL DEFAULT '', connection VARCHAR(100) NOT NULL DEFAULT '', create_time BIGINT UNSIGNED NULL)",
		"CREATE TABLE " + q("security_sensitive_data_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, sensitive_id INT UNSIGNED NOT NULL DEFAULT 0, data_table VARCHAR(100) NOT NULL DEFAULT '', primary_key VARCHAR(50) NOT NULL DEFAULT '', data_field VARCHAR(50) NOT NULL DEFAULT '', data_comment VARCHAR(50) NOT NULL DEFAULT '', id_value INT NOT NULL DEFAULT 0, `before` TEXT, `after` TEXT, ip VARCHAR(50) NOT NULL DEFAULT '', useragent VARCHAR(255) NOT NULL DEFAULT '', is_rollback TINYINT UNSIGNED NOT NULL DEFAULT 0, connection VARCHAR(100) NOT NULL DEFAULT '', create_time BIGINT UNSIGNED NULL)",
		"CREATE TABLE " + q("security_data_recycle") + " (id INT PRIMARY KEY, name VARCHAR(50) NOT NULL DEFAULT '', controller VARCHAR(100) NOT NULL DEFAULT '', controller_as VARCHAR(100) NOT NULL DEFAULT '', data_table VARCHAR(100) NOT NULL DEFAULT '', primary_key VARCHAR(50) NOT NULL DEFAULT '', status ENUM('0','1') NOT NULL DEFAULT '1', connection VARCHAR(100) NOT NULL DEFAULT '', update_time BIGINT UNSIGNED NULL, create_time BIGINT UNSIGNED NULL)",
		"CREATE TABLE " + q("security_sensitive_data") + " (id INT PRIMARY KEY, name VARCHAR(50) NOT NULL DEFAULT '', controller VARCHAR(100) NOT NULL DEFAULT '', controller_as VARCHAR(100) NOT NULL DEFAULT '', data_table VARCHAR(100) NOT NULL DEFAULT '', primary_key VARCHAR(50) NOT NULL DEFAULT '', data_fields TEXT, status ENUM('0','1') NOT NULL DEFAULT '1', connection VARCHAR(100) NOT NULL DEFAULT '', update_time BIGINT UNSIGNED NULL, create_time BIGINT UNSIGNED NULL)",
		"CREATE TABLE " + q("crud_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT UNSIGNED NOT NULL, descendant_id INT UNSIGNED NOT NULL, depth INT UNSIGNED NOT NULL DEFAULT 0, PRIMARY KEY (ancestor_id, descendant_id), KEY idx_descendant_ancestor (descendant_id, ancestor_id), KEY idx_ancestor_depth (ancestor_id, depth))",
		"CREATE TABLE " + q("admin_rule") + " (id INT UNSIGNED NOT NULL AUTO_INCREMENT, pid INT UNSIGNED NOT NULL DEFAULT 0, type VARCHAR(30) NOT NULL DEFAULT '', title VARCHAR(100) NOT NULL DEFAULT '', name VARCHAR(100) NOT NULL DEFAULT '', path VARCHAR(100) NOT NULL DEFAULT '', menu_type VARCHAR(30) NOT NULL DEFAULT '', component VARCHAR(255) NOT NULL DEFAULT '', weigh INT NOT NULL DEFAULT 0, status VARCHAR(10) NOT NULL DEFAULT '1', PRIMARY KEY (id))",
		"CREATE TABLE " + q("config") + " (id INT UNSIGNED NOT NULL AUTO_INCREMENT, name VARCHAR(30) NOT NULL DEFAULT '', `group` VARCHAR(30) NOT NULL DEFAULT '', title VARCHAR(50) NOT NULL DEFAULT '', tip VARCHAR(100) NOT NULL DEFAULT '', type VARCHAR(30) NOT NULL DEFAULT '', value LONGTEXT, content LONGTEXT, rule VARCHAR(100) NOT NULL DEFAULT '', extend VARCHAR(255) NOT NULL DEFAULT '', allow_del TINYINT UNSIGNED NOT NULL DEFAULT 0, weigh INT NOT NULL DEFAULT 0, PRIMARY KEY (id), UNIQUE KEY uq_config_name (name))",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO " + q("admin") + " VALUES (1,NULL),(2,1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("user") + " VALUES (10,2,'enable',1050),(20,1,'enable',250)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("user_money_log") + " VALUES (1,10,2,5,0,5),(2,20,1,3,0,3)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO "+q("security_data_recycle")+" (id,name,controller,controller_as,data_table,primary_key) VALUES (?, ?, ?, ?, ?, ?)", 5, "会员", "user/User.php", "user/user", "user", "id").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO "+q("security_sensitive_data")+" (id,name,controller,controller_as,data_table,primary_key,data_fields) VALUES (?, ?, ?, ?, ?, ?, ?)", 2, "会员数据", "user/User.php", "user/user", "user", "id", `{"username":"用户名","mobile":"手机号","status":"状态","email":"邮箱地址"}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO "+q("config")+" (id,name,`group`,title,tip,type,value,content,rule,extend,allow_del,weigh) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", 1, "config_group", "basics", "Config group", "", "array", `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"}]`, "", "required", "", 0, -1).Error; err != nil {
		t.Fatal(err)
	}
	registry := FrameworkMigrations()
	if len(registry) != 1 {
		t.Fatalf("current framework migration registry length=%d, want 1", len(registry))
	}
	if err := WithMigrationLock(db, "pinned-framework-registry", time.Second, func(pinned *gorm.DB) error {
		return registry[0].Up(pinned, config)
	}); err != nil {
		t.Fatal(err)
	}
	check := db.Session(&gorm.Session{NewDB: true})
	var invalid int64
	if err := check.Raw("SELECT COUNT(*) FROM " + q("user") + " u LEFT JOIN " + q("admin") + " a ON a.id=u.admin_id WHERE a.id IS NULL OR u.admin_id=0").Scan(&invalid).Error; err != nil || invalid != 0 {
		t.Fatalf("user owners invalid=%d err=%v", invalid, err)
	}
	if err := check.Raw("SELECT COUNT(*) FROM " + q("user_money_log") + " l JOIN " + q("user") + " u ON u.id=l.user_id WHERE l.admin_id<>u.admin_id").Scan(&invalid).Error; err != nil || invalid != 0 {
		t.Fatalf("money owners invalid=%d err=%v", invalid, err)
	}
	var uploadCount int64
	if err := check.Table(tableName(config, "config")).Where("`group` = ?", "upload").Count(&uploadCount).Error; err != nil || uploadCount != 6 {
		t.Fatalf("upload config rows=%d err=%v", uploadCount, err)
	}
}

func TestOfficialFailureRetryAndFrameworkPostVerifyOrder(t *testing.T) {
	db := getDB(t)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("retry_order_%d_", time.Now().UnixNano())}}
	requireNoError := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	requireNoError(BootstrapOfficialLedger(db, cfg))
	requireNoError(BootstrapFrameworkLedger(db, cfg))
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "migrations_framework")))
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "migrations")))
	})
	key := OfficialKey{Version: time.Now().UnixNano(), Name: "RetryOfficial"}
	fail := true
	official := []OfficialMigration{{Key: key, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error {
		if fail {
			fail = false
			return errors.New("official failure")
		}
		return nil
	}}}
	frameworkRan, schemaVerified, dataVerified := false, false, false
	framework := []FrameworkMigration{{Version: 1, MigrationName: "retry-framework", Up: func(*gorm.DB, *conf.Configuration) error { frameworkRan = true; return nil }, VerifySchema: func(*gorm.DB, *conf.Configuration) error { schemaVerified = true; return nil }, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { dataVerified = true; return nil }}}
	_, err := RunOfficialMigrations(db, cfg, official)
	requireNoErrorCheck := err != nil
	if !requireNoErrorCheck {
		t.Fatal("official failure was accepted")
	}
	if frameworkRan {
		t.Fatal("framework callback ran after official failure")
	}
	var count int64
	requireNoError(db.Table(tableName(cfg, "migrations")).Where("version=?", key.Version).Count(&count).Error)
	if count != 0 {
		t.Fatal("failed official migration was recorded")
	}
	_, err = RunOfficialMigrations(db, cfg, official)
	requireNoError(err)
	_, err = RunFrameworkMigrations(db, cfg, official, framework)
	requireNoError(err)
	if !frameworkRan || !schemaVerified || !dataVerified {
		t.Fatalf("framework order ran=%v schema=%v data=%v", frameworkRan, schemaVerified, dataVerified)
	}
}

func TestFrameworkBaselineRunsOnlyOnApplyAndStandingSchemaRunsOnEveryMigrate(t *testing.T) {
	db := getDB(t)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("baseline_%d_", time.Now().UnixNano())}}
	if err := BootstrapFrameworkLedger(db, cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + core.QuoteIdentifier(core.TableName(cfg, "migrations_framework")))
	})

	var upCalls, baselineCalls, schemaCalls int
	framework := FrameworkMigration{
		Version:       1,
		MigrationName: "baseline-contract",
		Up: func(*gorm.DB, *conf.Configuration) error {
			upCalls++
			return nil
		},
		VerifyBaseline: func(*gorm.DB, *conf.Configuration) error {
			baselineCalls++
			return nil
		},
		VerifySchema: func(*gorm.DB, *conf.Configuration) error {
			schemaCalls++
			return nil
		},
	}
	if _, err := RunFrameworkMigrations(db, cfg, nil, []FrameworkMigration{framework}); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, cfg, nil, []FrameworkMigration{framework}); err != nil {
		t.Fatal(err)
	}
	if upCalls != 1 || baselineCalls != 1 || schemaCalls != 2 {
		t.Fatalf("calls up=%d baseline=%d schema=%d", upCalls, baselineCalls, schemaCalls)
	}
}

func TestBusinessMoneyColumnOverrideSurvivesFrameworkStandingVerification(t *testing.T) {
	db := getDB(t)
	db, cfg := freshMigrationDatabase(t, db, fmt.Sprintf("biz_ovr_%d_", os.Getpid()))
	section := &migrationCriticalSection{}
	if _, err := runMigrationLifecycle(db, cfg, section); err != nil {
		t.Fatal(err)
	}
	moneyTable := core.TableName(cfg, "user_money_log")
	if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(moneyTable) + " MODIFY COLUMN " + core.QuoteIdentifier("money") + " decimal(12,2) NOT NULL DEFAULT 0.00").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, cfg, OfficialMigrations(), FrameworkMigrations()); err != nil {
		t.Fatal(err)
	}
	if err := FrameworkVerifyCurrent(db, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestDualTrackMySQLContractsAndAliases(t *testing.T) {
	db := getDB(t)
	config := &conf.Configuration{}
	config.Database.Prefix = "matrix_"
	for _, table := range []string{"matrix_migrations_framework", "matrix_migrations"} {
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
		defer db.Exec("DROP TABLE IF EXISTS `" + table + "`")
	}
	if err := db.Exec("CREATE TABLE `matrix_migrations` (`version` BIGINT UNSIGNED NOT NULL, `migration_name` VARCHAR(100) NULL DEFAULT NULL, `start_time` TIMESTAMP NULL DEFAULT NULL, `end_time` TIMESTAMP NULL DEFAULT NULL, `breakpoint` TINYINT(1) NOT NULL DEFAULT 0, PRIMARY KEY (`version`)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := BootstrapFrameworkLedger(db, config); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE `matrix_migrations_framework` ADD `unexpected` INT NULL").Error; err != nil {
		t.Fatal(err)
	}
	if err := ValidateFrameworkLedgerSchema(db, config); err == nil {
		t.Fatal("unexpected ledger column accepted")
	}
	if err := db.Exec("ALTER TABLE `matrix_migrations_framework` DROP COLUMN `unexpected`").Error; err != nil {
		t.Fatal(err)
	}
	if err := ValidateFrameworkLedgerSchema(db, config); err != nil {
		t.Fatal(err)
	}

	var calls []string
	framework := FrameworkMigration{Version: 1, MigrationName: "ordered",
		Up:                func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "up"); return nil },
		VerifySchema:      func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "schema"); return nil },
		VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "data"); return nil }}
	if _, err := RunFrameworkMigrations(db, config, nil, []FrameworkMigration{framework}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(calls); got != "[up schema data]" {
		t.Fatalf("missing order=%s", got)
	}
	calls = nil
	if _, err := RunFrameworkMigrations(db, config, nil, []FrameworkMigration{framework}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(calls); got != "[schema data]" {
		t.Fatalf("completed order=%s", got)
	}

	retryCalls := 0
	retry := FrameworkMigration{Version: 2, MigrationName: "retry", Up: func(*gorm.DB, *conf.Configuration) error { retryCalls++; return nil }}
	if err := InsertPendingFrameworkMigration(db, config, retry); err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, config, nil, []FrameworkMigration{framework, retry}); err != nil {
		t.Fatal(err)
	}
	if retryCalls != 1 {
		t.Fatalf("pending retry calls=%d", retryCalls)
	}
	noOp := func(*gorm.DB, *conf.Configuration) error { return nil }
	for _, collision := range []struct {
		name, want     string
		row, migration FrameworkMigration
	}{
		{"version", "framework version 3 collision", FrameworkMigration{Version: 3, MigrationName: "existing", Up: noOp}, FrameworkMigration{Version: 3, MigrationName: "other", Up: noOp}},
		{"migration name", "framework migration same-name collision", FrameworkMigration{Version: 4, MigrationName: "same-name", Up: noOp}, FrameworkMigration{Version: 5, MigrationName: "same-name", Up: noOp}},
	} {
		if err := InsertPendingFrameworkMigration(db, config, collision.row); err != nil {
			t.Fatal(err)
		}
		if _, err := RunFrameworkMigrations(db, config, nil, []FrameworkMigration{collision.migration}); err == nil || !strings.Contains(err.Error(), collision.want) {
			t.Fatalf("%s collision error=%v", collision.name, err)
		}
	}
	completed := FrameworkMigration{Version: 20, MigrationName: "complete-me", Up: noOp}
	if err := InsertPendingFrameworkMigration(db, config, completed); err != nil {
		t.Fatal(err)
	}
	if err := CompleteFrameworkMigration(db, config, completed); err != nil {
		t.Fatal("first completion:", err)
	}
	if err := CompleteFrameworkMigration(db, config, completed); err == nil {
		t.Fatal("second completion accepted")
	}
	for name, wrong := range map[string]FrameworkMigration{
		"migration name": {Version: 21, MigrationName: "wrong-name", Up: noOp},
		"missing name":   {Version: 22, MigrationName: "", Up: noOp},
	} {
		correct := wrong
		correct.MigrationName = "correct-name-" + name
		if err := InsertPendingFrameworkMigration(db, config, correct); err != nil {
			t.Fatal(err)
		}
		if err := CompleteFrameworkMigration(db, config, wrong); err == nil {
			t.Fatalf("%s mismatch accepted", name)
		}
		if err := CompleteFrameworkMigration(db, config, correct); err != nil {
			t.Fatal("correct completion:", err)
		}
	}

	official := []OfficialMigration{{Key: OfficialKey{Version: 1, Name: "Official"}, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	dependent := FrameworkMigration{Version: 7, MigrationName: "dependent", RequiresOfficial: []OfficialKey{official[0].Key}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
	for _, row := range []string{"", ", 'Official', NOW(6), NULL, 0", ", 'Wrong', NOW(6), NOW(6), 0"} {
		if row == "" {
			if _, err := RunFrameworkMigrations(db, config, official, []FrameworkMigration{dependent}); err == nil {
				t.Fatal("missing official accepted")
			}
		} else {
			if err := db.Exec("INSERT INTO `matrix_migrations` VALUES (1" + row + ")").Error; err != nil {
				t.Fatal(err)
			}
			if _, err := RunFrameworkMigrations(db, config, official, []FrameworkMigration{dependent}); err == nil {
				t.Fatal("pending/collision official accepted")
			}
			db.Exec("DELETE FROM `matrix_migrations`")
		}
	}
	if err := db.Exec("INSERT INTO `matrix_migrations` VALUES (1, 'Official', NOW(6), NOW(6), 0)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := RunFrameworkMigrations(db, config, official, []FrameworkMigration{dependent}); err != nil {
		t.Fatal("completed official rejected:", err)
	}

}

func TestDualTrackMySQLLedgerSchemaNegativeMatrix(t *testing.T) {
	db := getDB(t)
	variants := []string{"engine", "signed-version", "timestamp", "default", "missing-autoincrement", "missing-unique", "wrong-unique"}
	for i, variant := range variants {
		config := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("negative_%d_", i)}}
		table := config.Database.Prefix + "migrations_framework"
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
		if err := createLedgerVariant(db, table, variant); err != nil {
			t.Fatal(variant, err)
		}
		if err := ValidateFrameworkLedgerSchema(db, config); err == nil {
			t.Fatalf("%s schema accepted", variant)
		}
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createLedgerVariant(db *gorm.DB, table, variant string) error {
	version := "BIGINT UNSIGNED NOT NULL AUTO_INCREMENT"
	if variant == "signed-version" {
		version = "BIGINT NOT NULL AUTO_INCREMENT"
	}
	if variant == "missing-autoincrement" {
		version = "BIGINT UNSIGNED NOT NULL"
	}
	stamp := "TIMESTAMP(6)"
	if variant == "timestamp" {
		stamp = "TIMESTAMP"
	}
	start := "`start_time` " + stamp + " NOT NULL"
	if variant == "default" {
		start += " DEFAULT CURRENT_TIMESTAMP(6)"
	}
	unique := "UNIQUE KEY `uq_migrations_framework_migration_name` (`migration_name`)"
	if variant == "missing-unique" {
		unique = ""
	}
	if variant == "wrong-unique" {
		unique = "UNIQUE KEY `wrong_unique` (`migration_name`)"
	}
	engine := "InnoDB"
	if variant == "engine" {
		engine = "MyISAM"
	}
	return db.Exec("CREATE TABLE `" + table + "` (`version` " + version + ", `migration_name` VARCHAR(191) NOT NULL, " + start + ", `end_time` " + stamp + " NULL DEFAULT NULL, `breakpoint` TINYINT(1) NOT NULL DEFAULT 0, PRIMARY KEY (`version`)" + func() string {
		if unique == "" {
			return ""
		}
		return ", " + unique
	}() + ") ENGINE=" + engine).Error
}
