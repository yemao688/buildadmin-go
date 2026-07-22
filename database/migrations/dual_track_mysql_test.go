package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/local"
	"gorm.io/gorm"
)

func TestDualTrackMySQLLedgerAndLock(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	config := &conf.Configuration{}
	config.Database.Prefix = "phase1_"
	if err := db.Exec("DROP TABLE IF EXISTS `phase1_local_migrations`").Error; err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP TABLE IF EXISTS `phase1_local_migrations`")
	if err := BootstrapLocalLedger(db, config); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalLedgerSchema(db, config); err != nil {
		t.Fatal(err)
	}
	local := LocalMigration{Sequence: 1, ID: "retryable", Revision: 1, Up: func(_ *gorm.DB, _ *conf.Configuration) error { return nil }}
	if err := InsertPendingLocalMigration(db, config, local, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RunLocalMigrations(db, config, nil, []LocalMigration{local}); err != nil {
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

func TestLocalRegistryPinnedConnection0004Through0009(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	config := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("pinned_%d_", time.Now().UnixNano())}}
	q := func(logical string) string { return quoteIdentifier(tableName(config, logical)) }
	for _, logical := range []string{"admin", "user", "user_money_log", "user_score_log", "admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log", "admin_closure", "admin_hierarchy_lock"} {
		db.Exec("DROP TABLE IF EXISTS " + q(logical))
		table := q(logical)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + table) })
	}
	for _, ddl := range []string{
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, parent_id INT NULL)",
		"CREATE TABLE " + q("user") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, status VARCHAR(30) NOT NULL DEFAULT 'enable')",
		"CREATE TABLE " + q("user_money_log") + " (id INT PRIMARY KEY, user_id INT, admin_id INT UNSIGNED NOT NULL DEFAULT 0, money INT UNSIGNED, `before` INT UNSIGNED, `after` INT UNSIGNED)",
		"CREATE TABLE " + q("user_score_log") + " (id INT PRIMARY KEY, user_id INT, admin_id INT UNSIGNED NOT NULL DEFAULT 0, score INT UNSIGNED, `before` INT UNSIGNED, `after` INT UNSIGNED)",
		"CREATE TABLE " + q("admin_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_data_recycle_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, data TEXT)",
		"CREATE TABLE " + q("security_sensitive_data_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, `before` TEXT)",
		"CREATE TABLE " + q("security_data_recycle") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50))",
		"CREATE TABLE " + q("security_sensitive_data") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), data_fields TEXT)",
		"CREATE TABLE " + q("crud_log") + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT UNSIGNED NOT NULL, descendant_id INT UNSIGNED NOT NULL, depth INT UNSIGNED NOT NULL DEFAULT 0, PRIMARY KEY (ancestor_id, descendant_id), KEY idx_descendant_ancestor (descendant_id, ancestor_id), KEY idx_ancestor_depth (ancestor_id, depth))",
		"CREATE TABLE " + q("admin_hierarchy_lock") + " (id TINYINT UNSIGNED PRIMARY KEY)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO " + q("admin") + " VALUES (1,NULL),(2,1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("user") + " VALUES (10,2,'enable'),(20,0,'enable')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("user_money_log") + " VALUES (1,10,0,5,0,5),(2,20,0,3,0,3)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("user_score_log") + " VALUES (1,10,0,5,0,5)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + q("admin_hierarchy_lock") + " VALUES (1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := BootstrapOfficialLedger(db, config); err != nil {
		t.Fatal(err)
	}
	if err := BootstrapLocalLedger(db, config); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(config, "local_migrations")))
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(config, "migrations")))
	})
	for _, migration := range OfficialMigrations() {
		if err := db.Exec("INSERT INTO "+quoteIdentifier(tableName(config, "migrations"))+" (version, migration_name, start_time, end_time, breakpoint) VALUES (?, ?, NOW(6), NOW(6), 0)", migration.Key.Version, migration.Key.Name).Error; err != nil {
			t.Fatal(err)
		}
	}
	locals := LocalMigrations()[3:9]
	if err := WithMigrationLock(db, "pinned-local-registry", time.Second, func(pinned *gorm.DB) error {
		_, err := RunLocalMigrations(pinned, config, OfficialMigrations(), locals)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	check := db.Session(&gorm.Session{NewDB: true})
	var completed int64
	if err := check.Table(tableName(config, "local_migrations")).Where("sequence BETWEEN 4 AND 9 AND end_time IS NOT NULL").Count(&completed).Error; err != nil || completed != 6 {
		t.Fatalf("completed local 0004-0009=%d err=%v", completed, err)
	}
	var invalid int64
	if err := check.Raw("SELECT COUNT(*) FROM " + q("user") + " u LEFT JOIN " + q("admin") + " a ON a.id=u.admin_id WHERE a.id IS NULL OR u.admin_id=0").Scan(&invalid).Error; err != nil || invalid != 0 {
		t.Fatalf("user owners invalid=%d err=%v", invalid, err)
	}
	if err := check.Raw("SELECT COUNT(*) FROM " + q("user_money_log") + " l JOIN " + q("user") + " u ON u.id=l.user_id WHERE l.admin_id<>u.admin_id").Scan(&invalid).Error; err != nil || invalid != 0 {
		t.Fatalf("money owners invalid=%d err=%v", invalid, err)
	}
	localsForTest := local.Migrations(OfficialMigrations())
	if err := localsForTest[3].VerifySchema(check, config); err != nil {
		t.Fatal(err)
	}
	var owner int32
	if err := check.Table(tableName(config, "user")).Where("id=20").Pluck("admin_id", &owner).Error; err != nil || owner != 1 {
		t.Fatalf("historical user owner=%d err=%v", owner, err)
	}
	for _, item := range []struct{ table, column string }{{tableName(config, "user_money_log"), "money"}, {tableName(config, "user_score_log"), "score"}} {
		def, ok, err := core.MigrationColumnInfo(check, item.table, item.column)
		if err != nil || !ok {
			t.Fatalf("signed delta %s.%s=%#v ok=%v err=%v", item.table, item.column, def, ok, err)
		}
	}
	if err := localsForTest[5].VerifySchema(check, config); err != nil {
		t.Fatal(err)
	}
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		table := tableName(config, logical)
		def, ok, err := core.MigrationColumnInfo(check, table, "target_admin_id")
		if err != nil || !ok {
			t.Fatalf("target column %s err=%v", table, err)
		}
		has, first, err := core.MigrationIndexInfo(check, table, "idx_target_admin_id")
		if err != nil || !has || first != "target_admin_id" {
			t.Fatalf("target index %s has=%v first=%s err=%v", table, has, first, err)
		}
		_ = def
		for _, column := range []string{"legacy_unrecoverable", "is_committed"} {
			definition, ok, err := core.MigrationColumnInfo(check, table, column)
			if err != nil || !ok {
				t.Fatalf("security flag %s.%s=%#v ok=%v err=%v", table, column, definition, ok, err)
			}
		}
	}
	if err := localsForTest[6].VerifySchema(check, config); err != nil {
		t.Fatal(err)
	}
	if err := localsForTest[8].VerifySchema(check, config); err != nil {
		t.Fatal(err)
	}
	if err := localsForTest[9].VerifySchema(check, config); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialFailureRetryAndLocalPostVerifyOrder(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("retry_order_%d_", time.Now().UnixNano())}}
	requireNoError := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	requireNoError(BootstrapOfficialLedger(db, cfg))
	requireNoError(BootstrapLocalLedger(db, cfg))
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, "local_migrations")))
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
	localRan, schemaVerified, dataVerified := false, false, false
	local := []LocalMigration{{Sequence: 1, ID: "retry-local", Revision: 1, Up: func(*gorm.DB, *conf.Configuration) error { localRan = true; return nil }, VerifySchema: func(*gorm.DB, *conf.Configuration) error { schemaVerified = true; return nil }, VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { dataVerified = true; return nil }}}
	_, err := RunOfficialMigrations(db, cfg, official)
	requireNoErrorCheck := err != nil
	if !requireNoErrorCheck {
		t.Fatal("official failure was accepted")
	}
	if localRan {
		t.Fatal("local callback ran after official failure")
	}
	var count int64
	requireNoError(db.Table(tableName(cfg, "migrations")).Where("version=?", key.Version).Count(&count).Error)
	if count != 0 {
		t.Fatal("failed official migration was recorded")
	}
	_, err = RunOfficialMigrations(db, cfg, official)
	requireNoError(err)
	_, err = RunLocalMigrations(db, cfg, official, local)
	requireNoError(err)
	if !localRan || !schemaVerified || !dataVerified {
		t.Fatalf("local order ran=%v schema=%v data=%v", localRan, schemaVerified, dataVerified)
	}
}

func TestDualTrackMySQLContractsAndAliases(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	config := &conf.Configuration{}
	config.Database.Prefix = "matrix_"
	for _, table := range []string{"matrix_local_migrations", "matrix_migrations"} {
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
		defer db.Exec("DROP TABLE IF EXISTS `" + table + "`")
	}
	if err := db.Exec("CREATE TABLE `matrix_migrations` (`version` BIGINT NOT NULL PRIMARY KEY, `migration_name` VARCHAR(100), `start_time` TIMESTAMP NULL, `end_time` TIMESTAMP NULL) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := BootstrapLocalLedger(db, config); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE `matrix_local_migrations` ADD `unexpected` INT NULL").Error; err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalLedgerSchema(db, config); err == nil {
		t.Fatal("unexpected ledger column accepted")
	}
	if err := db.Exec("ALTER TABLE `matrix_local_migrations` DROP COLUMN `unexpected`").Error; err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalLedgerSchema(db, config); err != nil {
		t.Fatal(err)
	}

	var calls []string
	local := LocalMigration{Sequence: 1, ID: "ordered", Revision: 1,
		Up:                func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "up"); return nil },
		VerifySchema:      func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "schema"); return nil },
		VerifyUpgradeData: func(*gorm.DB, *conf.Configuration) error { calls = append(calls, "data"); return nil }}
	if _, err := RunLocalMigrations(db, config, nil, []LocalMigration{local}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(calls); got != "[up schema data]" {
		t.Fatalf("missing order=%s", got)
	}
	calls = nil
	if _, err := RunLocalMigrations(db, config, nil, []LocalMigration{local}); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(calls); got != "[schema data]" {
		t.Fatalf("completed order=%s", got)
	}

	retryCalls := 0
	retry := LocalMigration{Sequence: 2, ID: "retry", Revision: 1, Up: func(*gorm.DB, *conf.Configuration) error { retryCalls++; return nil }}
	if err := InsertPendingLocalMigration(db, config, retry, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RunLocalMigrations(db, config, nil, []LocalMigration{local, retry}); err != nil {
		t.Fatal(err)
	}
	if retryCalls != 1 {
		t.Fatalf("pending retry calls=%d", retryCalls)
	}
	noOp := func(*gorm.DB, *conf.Configuration) error { return nil }
	for _, collision := range []struct {
		name, want     string
		row, migration LocalMigration
	}{
		{"sequence", "local sequence 3 collision", LocalMigration{Sequence: 3, ID: "existing", Revision: 1, Up: noOp}, LocalMigration{Sequence: 3, ID: "other", Revision: 1, Up: noOp}},
		{"id", "local migration same-id collision", LocalMigration{Sequence: 4, ID: "same-id", Revision: 1, Up: noOp}, LocalMigration{Sequence: 5, ID: "same-id", Revision: 1, Up: noOp}},
		{"revision", "local sequence 6 collision", LocalMigration{Sequence: 6, ID: "same-revision", Revision: 1, Up: noOp}, LocalMigration{Sequence: 6, ID: "same-revision", Revision: 2, Up: noOp}},
	} {
		if err := InsertPendingLocalMigration(db, config, collision.row, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := RunLocalMigrations(db, config, nil, []LocalMigration{collision.migration}); err == nil || !strings.Contains(err.Error(), collision.want) {
			t.Fatalf("%s collision error=%v", collision.name, err)
		}
	}
	completed := LocalMigration{Sequence: 20, ID: "complete-me", Revision: 9, Up: noOp}
	if err := InsertPendingLocalMigration(db, config, completed, nil); err != nil {
		t.Fatal(err)
	}
	if err := CompleteLocalMigration(db, config, completed); err != nil {
		t.Fatal("first completion:", err)
	}
	if err := CompleteLocalMigration(db, config, completed); err == nil {
		t.Fatal("second completion accepted")
	}
	for name, wrong := range map[string]LocalMigration{
		"id":       {Sequence: 21, ID: "wrong-id", Revision: 1, Up: noOp},
		"revision": {Sequence: 22, ID: "wrong-revision", Revision: 1, Up: noOp},
		"sequence": {Sequence: 23, ID: "wrong-sequence", Revision: 1, Up: noOp},
	} {
		correct := wrong
		if name == "id" {
			correct.ID = "correct-id"
		}
		if name == "revision" {
			correct.Revision = 2
		}
		if name == "sequence" {
			correct.Sequence = 24
		}
		if err := InsertPendingLocalMigration(db, config, correct, nil); err != nil {
			t.Fatal(err)
		}
		if err := CompleteLocalMigration(db, config, wrong); err == nil {
			t.Fatalf("%s mismatch accepted", name)
		}
		if err := CompleteLocalMigration(db, config, correct); err != nil {
			t.Fatal("correct completion:", err)
		}
	}

	official := []OfficialMigration{{Key: OfficialKey{Version: 1, Name: "Official"}, Source: "test", Up: func(*gorm.DB, *conf.Configuration) error { return nil }}}
	dependent := LocalMigration{Sequence: 7, ID: "dependent", Revision: 1, RequiresOfficial: []OfficialKey{official[0].Key}, Up: func(*gorm.DB, *conf.Configuration) error { return nil }}
	for _, row := range []string{"", ", 'Official', NOW(6), NULL", ", 'Wrong', NOW(6), NOW(6)"} {
		if row == "" {
			if _, err := RunLocalMigrations(db, config, official, []LocalMigration{dependent}); err == nil {
				t.Fatal("missing official accepted")
			}
		} else {
			if err := db.Exec("INSERT INTO `matrix_migrations` VALUES (1" + row + ")").Error; err != nil {
				t.Fatal(err)
			}
			if _, err := RunLocalMigrations(db, config, official, []LocalMigration{dependent}); err == nil {
				t.Fatal("pending/collision official accepted")
			}
			db.Exec("DELETE FROM `matrix_migrations`")
		}
	}
	if err := db.Exec("INSERT INTO `matrix_migrations` VALUES (1, 'Official', NOW(6), NOW(6))").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := RunLocalMigrations(db, config, official, []LocalMigration{dependent}); err != nil {
		t.Fatal("completed official rejected:", err)
	}

}

func TestDualTrackMySQLLedgerSchemaNegativeMatrix(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	variants := []string{"engine", "signed-revision", "timestamp", "default", "missing-unique", "wrong-unique"}
	for i, variant := range variants {
		config := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("negative_%d_", i)}}
		table := config.Database.Prefix + "local_migrations"
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
		if err := createLedgerVariant(db, table, variant); err != nil {
			t.Fatal(variant, err)
		}
		if err := ValidateLocalLedgerSchema(db, config); err == nil {
			t.Fatalf("%s schema accepted", variant)
		}
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createLedgerVariant(db *gorm.DB, table, variant string) error {
	revision := "BIGINT UNSIGNED"
	if variant == "signed-revision" {
		revision = "BIGINT"
	}
	stamp := "TIMESTAMP(6)"
	if variant == "timestamp" {
		stamp = "TIMESTAMP"
	}
	start := "`start_time` " + stamp + " NOT NULL"
	if variant == "default" {
		start += " DEFAULT CURRENT_TIMESTAMP(6)"
	}
	unique := "UNIQUE KEY `uq_local_migrations_id` (`migration_id`)"
	if variant == "missing-unique" {
		unique = ""
	}
	if variant == "wrong-unique" {
		unique = "UNIQUE KEY `wrong_unique` (`migration_id`)"
	}
	engine := "InnoDB"
	if variant == "engine" {
		engine = "MyISAM"
	}
	return db.Exec("CREATE TABLE `" + table + "` (`sequence` BIGINT UNSIGNED NOT NULL, `migration_id` VARCHAR(191) NOT NULL, `revision` " + revision + " NOT NULL, " + start + ", `end_time` " + stamp + " NULL DEFAULT NULL, `adopted_from` VARCHAR(191) NULL DEFAULT NULL, PRIMARY KEY (`sequence`)" + func() string {
		if unique == "" {
			return ""
		}
		return ", " + unique
	}() + ") ENGINE=" + engine).Error
}
