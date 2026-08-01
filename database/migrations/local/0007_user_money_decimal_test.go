package local

import (
	"testing"

	"go-build-admin/app/pkg/testutil"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func TestUserMoneyDecimalConvertsCentsAndIsIdempotent(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "money_decimal_")
	userTable := core.TableName(config, "user")
	logTable := core.TableName(config, "user_money_log")
	quotedUserTable := core.QuoteIdentifier(userTable)
	quotedLogTable := core.QuoteIdentifier(logTable)

	if err := db.Exec("CREATE TABLE " + quotedUserTable + " (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY," +
		"`money` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '余额'" +
		") ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + quotedLogTable + " (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY," +
		"`money` int(11) NOT NULL DEFAULT 0 COMMENT '变更余额'," +
		"`before` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '变更前余额'," +
		"`after` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '变更后余额'" +
		") ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + quotedUserTable + " (`id`, `money`) VALUES (1, 1050), (2, 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + quotedLogTable + " (`id`, `money`, `before`, `after`) VALUES (1, -250, 1050, 800), (2, 0, 0, 0)").Error; err != nil {
		t.Fatal(err)
	}

	if err := userMoneyDecimalUp(db, config); err != nil {
		t.Fatal(err)
	}
	assertUserMoneyDecimalTypes(t, db, config)
	if err := verifyUserMoneyDecimalBaseline(db, config); err != nil {
		t.Fatal(err)
	}
	assertDecimalValue(t, db, userTable, "money", 1, "10.50")
	assertDecimalValue(t, db, userTable, "money", 2, "0.00")
	assertDecimalValue(t, db, logTable, "money", 1, "-2.50")
	assertDecimalValue(t, db, logTable, "money", 2, "0.00")
	assertDecimalValue(t, db, logTable, "before", 1, "10.50")
	assertDecimalValue(t, db, logTable, "before", 2, "0.00")
	assertDecimalValue(t, db, logTable, "after", 1, "8.00")
	assertDecimalValue(t, db, logTable, "after", 2, "0.00")

	if err := userMoneyDecimalUp(db, config); err != nil {
		t.Fatal(err)
	}
	assertUserMoneyDecimalTypes(t, db, config)
	assertDecimalValue(t, db, userTable, "money", 1, "10.50")
	assertDecimalValue(t, db, logTable, "money", 1, "-2.50")
	assertDecimalValue(t, db, logTable, "before", 1, "10.50")
	assertDecimalValue(t, db, logTable, "after", 1, "8.00")
}

func TestUserMoneyDecimalPreservesAlreadyYuanSources(t *testing.T) {
	db, config := testutil.OpenFixtureDatabase(t, "money_decimal_mixed_")
	userTable := core.TableName(config, "user")
	logTable := core.TableName(config, "user_money_log")
	quotedUserTable := core.QuoteIdentifier(userTable)
	quotedLogTable := core.QuoteIdentifier(logTable)

	if err := db.Exec("CREATE TABLE " + quotedUserTable + " (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY," +
		"`money` double NOT NULL DEFAULT 0 COMMENT '余额'" +
		") ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + quotedLogTable + " (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY," +
		"`money` int(11) NOT NULL DEFAULT 0 COMMENT '变更余额'," +
		"`before` decimal(10,2) NOT NULL DEFAULT 0.00 COMMENT '变更前余额'," +
		"`after` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '变更后余额'" +
		") ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + quotedUserTable + " (`id`, `money`) VALUES (1, 10.5), (2, 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + quotedLogTable + " (`id`, `money`, `before`, `after`) VALUES (1, -250, 8.00, 800), (2, 0, 0.00, 0)").Error; err != nil {
		t.Fatal(err)
	}

	if err := userMoneyDecimalUp(db, config); err != nil {
		t.Fatal(err)
	}
	assertUserMoneyDecimalTypes(t, db, config)
	if err := verifyUserMoneyDecimalBaseline(db, config); err != nil {
		t.Fatal(err)
	}
	assertDecimalValue(t, db, userTable, "money", 1, "10.50")
	assertDecimalValue(t, db, logTable, "money", 1, "-2.50")
	assertDecimalValue(t, db, logTable, "before", 1, "8.00")
	assertDecimalValue(t, db, logTable, "after", 1, "8.00")

	if err := userMoneyDecimalUp(db, config); err != nil {
		t.Fatal(err)
	}
	assertUserMoneyDecimalTypes(t, db, config)
	assertDecimalValue(t, db, userTable, "money", 1, "10.50")
	assertDecimalValue(t, db, logTable, "money", 1, "-2.50")
	assertDecimalValue(t, db, logTable, "before", 1, "8.00")
	assertDecimalValue(t, db, logTable, "after", 1, "8.00")
}

func assertUserMoneyDecimalTypes(t *testing.T, db *gorm.DB, config *conf.Configuration) {
	t.Helper()
	for _, item := range userMoneyDecimalColumns {
		definition, ok, err := core.MigrationColumnInfo(db, core.TableName(config, item.table), item.column)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || definition.ColumnType != userMoneyDecimalType {
			t.Fatalf("%s.%s type=%q exists=%v, want %s", item.table, item.column, definition.ColumnType, ok, userMoneyDecimalType)
		}
	}
}

func assertDecimalValue(t *testing.T, db *gorm.DB, table, column string, id int, want string) {
	t.Helper()
	var got string
	if err := db.Raw("SELECT CAST("+core.QuoteIdentifier(column)+" AS CHAR) FROM "+core.QuoteIdentifier(table)+" WHERE `id` = ?", id).Scan(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s.%s id=%d value=%q, want %q", table, column, id, got, want)
	}
}
