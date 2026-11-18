package crudgen

import (
	"strings"
	"testing"

	"buildadmin-go/internal/conf"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// spec-only 回退与守卫的提前返回也在生成锁内：漏注册 release 会让常驻进程
// （CrudService.Delete）永久卡在 another generation is in progress。
// 非并行：generationMu 是进程级状态，并行会与其他生成用例互相抢锁。
//
//nolint:paralleltest // 进程级 generationMu 必须在同一时刻独占
func TestDeleteFromSpecReleasesLockOnEarlyReturn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "ba_", SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE ba_crud_log (id integer primary key autoincrement, table_name text, status text, comment text, create_time integer)").Error; err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"

	if err := DeleteFromSpec(db, cfg, "user"); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("first delete error = %v", err)
	}
	// 锁若泄漏，这里会变成 "another generation is in progress"。
	if err := DeleteFromSpec(db, cfg, "user"); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("second delete error = %v (generation lock leaked?)", err)
	}
}
