package migrations

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func TestLocalVerifiersFailClosedOnMissingCoreSchema(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("missing_contract_%d_", time.Now().UnixNano())}}
	checks := []struct {
		name string
		fn   func(*gorm.DB, *conf.Configuration) error
	}{
		{"status", verifyStatusContract}, {"hierarchy", verifyHierarchyContract}, {"attachment", verifyAttachmentContract},
		{"user_owner", verifyUserOwnerContract}, {"security_owner", verifySecurityOwnerContract}, {"signed_delta", verifySignedDeltaContract},
		{"target", verifyTargetContract}, {"legacy_target", verifyLegacyTargetContract}, {"commit", verifyCommitContract}, {"security_rule", verifySecurityRuleContract},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) { require.Error(t, check.fn(db, cfg)) })
	}
}

func TestLocalVerifiersRejectConcreteWrongContracts(t *testing.T) {
	db := getDB()
	if db == nil {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	q := func(cfg *conf.Configuration, logical string) string { return quoteIdentifier(tableName(cfg, logical)) }
	newConfig := func(label string) *conf.Configuration {
		return &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("wrong_%s_%d_", label, time.Now().UnixNano())}}
	}

	signed := newConfig("signed")
	for _, logical := range []string{"admin", "user_money_log", "user_score_log"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+q(signed, logical)).Error)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(signed, logical)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(signed, "admin")+" (id INT PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(signed, "user_money_log")+" (id INT PRIMARY KEY, money VARCHAR(20) NOT NULL DEFAULT '')").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(signed, "user_score_log")+" (id INT PRIMARY KEY, score BIGINT NOT NULL DEFAULT 0)").Error)
	require.Error(t, verifySignedDeltaContract(db.Session(&gorm.Session{NewDB: true}), signed))

	flags := newConfig("flags")
	for _, logical := range []string{"admin", "security_data_recycle_log", "security_sensitive_data_log"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+q(flags, logical)).Error)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(flags, logical)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(flags, "admin")+" (id INT PRIMARY KEY)").Error)
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		require.NoError(t, db.Exec("CREATE TABLE "+q(flags, logical)+" (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, legacy_unrecoverable TINYINT NULL DEFAULT 1, is_committed TINYINT NOT NULL DEFAULT 0, KEY idx_target_admin_id (target_admin_id))").Error)
	}
	require.Error(t, verifyLegacyTargetContract(db.Session(&gorm.Session{NewDB: true}), flags))
	require.Error(t, verifyCommitContract(db.Session(&gorm.Session{NewDB: true}), flags))

	seed := newConfig("seed")
	for _, logical := range []string{"admin", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+q(seed, logical)).Error)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(seed, logical)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(seed, "admin")+" (id INT PRIMARY KEY)").Error)
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		require.NoError(t, db.Exec("CREATE TABLE "+q(seed, logical)+" (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, legacy_unrecoverable TINYINT UNSIGNED NOT NULL DEFAULT 0, is_committed TINYINT UNSIGNED NOT NULL DEFAULT 0, KEY idx_target_admin_id (target_admin_id))").Error)
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(seed, "security_data_recycle")+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(seed, "security_sensitive_data")+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), data_fields TEXT)").Error)
	require.NoError(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), seed))
	require.NoError(t, db.Exec("INSERT INTO "+q(seed, "security_data_recycle")+" VALUES (5,'会员','user/User.php','user/user','user','id'),(6,'会员','user/User.php','user/user','user','id')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(seed, "security_sensitive_data")+" VALUES (2,'会员数据','user/User.php','user/user','user','id','{\"username\":\"用户名\"}'),(3,'会员数据','user/User.php','user/user','user','id','{\"username\":\"用户名\"}')").Error)
	require.NoError(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), seed))
	require.NoError(t, db.Exec("DELETE FROM "+q(seed, "security_data_recycle")+" WHERE id=6").Error)
	require.NoError(t, db.Exec("DELETE FROM "+q(seed, "security_sensitive_data")+" WHERE id=3").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(seed, "security_data_recycle")+" VALUES (1,'管理员','auth/Admin.php','auth/admin','admin','id')").Error)
	require.Error(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), seed))
	require.NoError(t, db.Exec("DELETE FROM "+q(seed, "security_data_recycle")+" WHERE id=1").Error)
	require.NoError(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), seed))
}
