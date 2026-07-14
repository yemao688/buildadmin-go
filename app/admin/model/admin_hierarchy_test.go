package model

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"go-build-admin/conf"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func openAdminHierarchyTestDB(t *testing.T, prefix string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   prefix,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func hierarchyConfig(prefix string) *conf.Configuration {
	return &conf.Configuration{Database: conf.Database{Prefix: prefix}}
}

func createAdminForHierarchy(t *testing.T, db *gorm.DB, username string) Admin {
	t.Helper()
	a := Admin{Username: username, Status: "enable"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("create admin %s: %v", username, err)
	}
	return a
}

func closureRows(t *testing.T, db *gorm.DB) []AdminClosure {
	t.Helper()
	var rows []AdminClosure
	if err := db.Model(&AdminClosure{}).Order("ancestor_id, descendant_id").Find(&rows).Error; err != nil {
		t.Fatalf("list closure rows: %v", err)
	}
	return rows
}

func ancestorIDs(rows []AdminClosure, descendantID int32) []int32 {
	var ids []int32
	for _, r := range rows {
		if r.DescendantID == descendantID {
			ids = append(ids, r.AncestorID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func newAdminHierarchy(prefix string) *AdminHierarchy {
	return NewAdminHierarchy(hierarchyConfig(prefix))
}

func ensureAdminDefaults(t *testing.T, db *gorm.DB) {
	t.Helper()
	// AdminModel.Add omits login_failure and last_login_ip. Align the runtime
	// test schema with the migration model defaults so the insert succeeds.
	if err := db.Exec("ALTER TABLE `ba_admin` ALTER COLUMN `login_failure` SET DEFAULT 0").Error; err != nil {
		t.Fatalf("set login_failure default: %v", err)
	}
	if err := db.Exec("ALTER TABLE `ba_admin` MODIFY COLUMN `last_login_ip` VARCHAR(50) NOT NULL DEFAULT ''").Error; err != nil {
		t.Fatalf("set last_login_ip default: %v", err)
	}
}

func ensureLockTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("CREATE TABLE IF NOT EXISTS `ba_admin_hierarchy_lock` (" +
		"`id` tinyint(3) unsigned NOT NULL," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
		t.Fatalf("create lock table: %v", err)
	}
	if err := db.Exec("INSERT IGNORE INTO `ba_admin_hierarchy_lock` (id) VALUES (1)").Error; err != nil {
		t.Fatalf("seed lock row: %v", err)
	}
}

func assertAncestors(t *testing.T, db *gorm.DB, descendantID int32, want []int32) {
	t.Helper()
	got := ancestorIDs(closureRows(t, db), descendantID)
	if len(got) != len(want) {
		t.Fatalf("ancestors of %d = %v, want %v", descendantID, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ancestors of %d = %v, want %v", descendantID, got, want)
		}
	}
}

func adminParentID(t *testing.T, db *gorm.DB, id int32) *int32 {
	t.Helper()
	var parent *int32
	if err := db.Model(&Admin{}).Where("id = ?", id).Select("parent_id").Scan(&parent).Error; err != nil {
		t.Fatalf("read parent_id: %v", err)
	}
	return parent
}

func TestAdminHierarchyLinkNewNode(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "root")
	b := createAdminForHierarchy(t, db, "child")

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(context.Background(), tx, a.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID)
	}); err != nil {
		t.Fatalf("link nodes: %v", err)
	}

	assertAncestors(t, db, a.ID, []int32{a.ID})
	assertAncestors(t, db, b.ID, []int32{a.ID, b.ID})
	if got := adminParentID(t, db, b.ID); got == nil || *got != a.ID {
		t.Fatalf("admin.parent_id of B = %v, want %d", got, a.ID)
	}
}

func TestAdminHierarchyLinkNewNodeOrphanParent(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "orphan")
	missing := int32(9999)

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, a.ID, &missing)
	})
	if !errors.Is(err, ErrHierarchyOrphanParent) {
		t.Fatalf("expected ErrHierarchyOrphanParent, got %v", err)
	}

	if got := countClosureRows(t, db); got != 0 {
		t.Fatalf("closure rows = %d, want 0", got)
	}
}

func TestAdminHierarchyMoveRootToRoot(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, a.ID, nil)
	}); err != nil {
		t.Fatalf("link root: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, a.ID, nil)
	}); err != nil {
		t.Fatalf("root-to-root move: %v", err)
	}

	assertAncestors(t, db, a.ID, []int32{a.ID})
}

func TestAdminHierarchyMoveRootUnderParent(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(context.Background(), tx, a.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(context.Background(), tx, b.ID, nil)
	}); err != nil {
		t.Fatalf("link roots: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, b.ID, &a.ID)
	}); err != nil {
		t.Fatalf("move root under parent: %v", err)
	}

	assertAncestors(t, db, b.ID, []int32{a.ID, b.ID})
	if got := adminParentID(t, db, b.ID); got == nil || *got != a.ID {
		t.Fatalf("admin.parent_id of B = %v, want %d", got, a.ID)
	}
}

func TestAdminHierarchyMoveSubtree(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")
	c := createAdminForHierarchy(t, db, "C")
	d := createAdminForHierarchy(t, db, "D")

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, op := range []func() error{
			func() error { return h.LinkNewNode(context.Background(), tx, a.ID, nil) },
			func() error { return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, c.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, d.ID, &b.ID) },
		} {
			if err := op(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, b.ID, &c.ID)
	}); err != nil {
		t.Fatalf("move B under C: %v", err)
	}

	assertAncestors(t, db, b.ID, []int32{a.ID, b.ID, c.ID})
	assertAncestors(t, db, d.ID, []int32{a.ID, b.ID, c.ID, d.ID})
	assertAncestors(t, db, c.ID, []int32{a.ID, c.ID})
	if got := adminParentID(t, db, b.ID); got == nil || *got != c.ID {
		t.Fatalf("admin.parent_id of B = %v, want %d", got, c.ID)
	}
}

func TestAdminHierarchyMoveRejectsInvalid(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")
	c := createAdminForHierarchy(t, db, "C")

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, op := range []func() error{
			func() error { return h.LinkNewNode(context.Background(), tx, a.ID, nil) },
			func() error { return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, c.ID, &b.ID) },
		} {
			if err := op(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, a.ID, &a.ID)
	}); !errors.Is(err, ErrHierarchySelfMove) {
		t.Fatalf("self move: got %v, want ErrHierarchySelfMove", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, a.ID, &c.ID)
	}); !errors.Is(err, ErrHierarchyDescendantMove) {
		t.Fatalf("descendant move: got %v, want ErrHierarchyDescendantMove", err)
	}
	missing := int32(9999)
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, a.ID, &missing)
	}); !errors.Is(err, ErrHierarchyOrphanParent) {
		t.Fatalf("orphan move: got %v, want ErrHierarchyOrphanParent", err)
	}
}

func TestAdminHierarchyMoveRejectsInconsistentParent(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(context.Background(), tx, a.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Corrupt admin.parent_id so it no longer matches the closure depth=1 edge.
	if err := db.Model(&Admin{}).Where("id = ?", b.ID).Update("parent_id", a.ID+1000).Error; err != nil {
		t.Fatalf("corrupt parent_id: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, b.ID, nil)
	}); !errors.Is(err, ErrHierarchyIntegrity) {
		t.Fatalf("expected ErrHierarchyIntegrity, got %v", err)
	}
}

func TestAdminHierarchyConcurrentMoves(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")
	c := createAdminForHierarchy(t, db, "C")
	d := createAdminForHierarchy(t, db, "D")

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, op := range []func() error{
			func() error { return h.LinkNewNode(context.Background(), tx, a.ID, nil) },
			func() error { return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, c.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, d.ID, &a.ID) },
		} {
			if err := op(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Two valid moves that touch overlapping closure rows; the lock must serialize them.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errs[0] = db.Transaction(func(tx *gorm.DB) error {
			return h.MoveSubtree(ctx, tx, b.ID, &c.ID)
		})
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		errs[1] = db.Transaction(func(tx *gorm.DB) error {
			return h.MoveSubtree(ctx, tx, d.ID, &b.ID)
		})
	}()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent move %d failed: %v", i, err)
		}
	}

	rows := closureRows(t, db)
	for _, descendant := range []int32{a.ID, b.ID, c.ID, d.ID} {
		got := ancestorIDs(rows, descendant)
		if len(got) == 0 {
			t.Fatalf("admin %d lost all closure rows", descendant)
		}
	}
	if got := adminParentID(t, db, b.ID); got == nil || *got != c.ID {
		t.Fatalf("B parent = %v, want %d", got, c.ID)
	}
	if got := adminParentID(t, db, d.ID); got == nil || *got != b.ID {
		t.Fatalf("D parent = %v, want %d", got, b.ID)
	}
}

func TestAdminHierarchyModelAddWithParentAndClosure(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	h := newAdminHierarchy("ba_")
	parent := createAdminForHierarchy(t, db, "parent")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, parent.ID, nil)
	}); err != nil {
		t.Fatalf("link parent: %v", err)
	}

	model := NewAdminModel(db, hierarchyConfig("ba_"))
	child := Admin{Username: "child", Status: "enable", ParentID: &parent.ID}
	if err := model.Add(nil, child, []string{"1"}); err != nil {
		t.Fatalf("add child: %v", err)
	}

	var created Admin
	if err := db.Where("username = ?", "child").First(&created).Error; err != nil {
		t.Fatalf("find created admin: %v", err)
	}
	if got := adminParentID(t, db, created.ID); got == nil || *got != parent.ID {
		t.Fatalf("created admin parent_id = %v, want %d", got, parent.ID)
	}
	assertAncestors(t, db, created.ID, []int32{parent.ID, created.ID})
}

func TestAdminHierarchyModelAddRollbackOnClosureFailure(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	model := NewAdminModel(db, hierarchyConfig("ba_"))
	missing := int32(9999)
	child := Admin{Username: "rollback", Status: "enable", ParentID: &missing}
	if err := model.Add(nil, child, []string{"1"}); err == nil {
		t.Fatal("expected add to fail due to orphan parent")
	}

	var count int64
	if err := db.Model(&Admin{}).Where("username = ?", "rollback").Count(&count).Error; err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if count != 0 {
		t.Fatalf("admin rows = %d, want 0 after rollback", count)
	}
	if got := countClosureRows(t, db); got != 0 {
		t.Fatalf("closure rows = %d, want 0 after rollback", got)
	}
	if got := countGroupAccessRows(t, db); got != 0 {
		t.Fatalf("group access rows = %d, want 0 after rollback", got)
	}
}

func lockTable(t *testing.T, db *gorm.DB) string {
	t.Helper()
	return "ba_admin_hierarchy_lock"
}

func assertLockRowExists(t *testing.T, db *gorm.DB) {
	t.Helper()
	var id uint8
	if err := db.Raw("SELECT id FROM `ba_admin_hierarchy_lock` WHERE id = 1").Scan(&id).Error; err != nil {
		t.Fatalf("lock row missing: %v", err)
	}
	if id != 1 {
		t.Fatalf("lock row id = %d, want 1", id)
	}
}

func TestAdminHierarchyLockRowMissingFailsClosed(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := db.Exec("DELETE FROM `ba_admin_hierarchy_lock`").Error; err != nil {
		t.Fatalf("delete lock row: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "nolock")
	err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, a.ID, nil)
	})
	if err == nil {
		t.Fatal("expected error when lock row is missing")
	}
}

func TestAdminHierarchyClosureOnlyRealAdmins(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")
	c := createAdminForHierarchy(t, db, "C")
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, op := range []func() error{
			func() error { return h.LinkNewNode(context.Background(), tx, a.ID, nil) },
			func() error { return h.LinkNewNode(context.Background(), tx, b.ID, &a.ID) },
			func() error { return h.LinkNewNode(context.Background(), tx, c.ID, &b.ID) },
		} {
			if err := op(); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	rows := closureRows(t, db)
	for _, r := range rows {
		for _, id := range []int32{r.AncestorID, r.DescendantID} {
			var exists int64
			if err := db.Model(&Admin{}).Where("id = ?", id).Count(&exists).Error; err != nil {
				t.Fatalf("count admin %d: %v", id, err)
			}
			if exists == 0 {
				t.Fatalf("closure row (%d,%d,%d) references missing admin %d", r.AncestorID, r.DescendantID, r.Depth, id)
			}
		}
		if r.AncestorID == 0 || r.DescendantID == 0 {
			t.Fatalf("closure row contains technical id 0: (%d,%d,%d)", r.AncestorID, r.DescendantID, r.Depth)
		}
	}

	var depthZeroCount int64
	if err := db.Model(&AdminClosure{}).Where("depth = ?", 0).Count(&depthZeroCount).Error; err != nil {
		t.Fatalf("count depth=0: %v", err)
	}
	var adminCount int64
	if err := db.Model(&Admin{}).Count(&adminCount).Error; err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if depthZeroCount != adminCount {
		t.Fatalf("depth=0 rows = %d, admin rows = %d", depthZeroCount, adminCount)
	}
	assertLockRowExists(t, db)
}

func TestAdminHierarchyMoveRejectsTwoDirectParents(t *testing.T) {
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	h := newAdminHierarchy("ba_")
	a := createAdminForHierarchy(t, db, "A")
	b := createAdminForHierarchy(t, db, "B")
	x := createAdminForHierarchy(t, db, "X")

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(context.Background(), tx, a.ID, nil); err != nil {
			return err
		}
		if err := h.LinkNewNode(context.Background(), tx, b.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(context.Background(), tx, x.ID, &a.ID)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Corrupt closure: insert a second depth=1 parent for x.
	if err := db.Exec("INSERT INTO `ba_admin_closure` (ancestor_id, descendant_id, depth) VALUES (?, ?, 1)", b.ID, x.ID).Error; err != nil {
		t.Fatalf("corrupt closure: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtree(context.Background(), tx, x.ID, nil)
	})
	if !errors.Is(err, ErrHierarchyIntegrity) {
		t.Fatalf("expected ErrHierarchyIntegrity, got %v", err)
	}
}

func countClosureRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&AdminClosure{}).Count(&count).Error; err != nil {
		t.Fatalf("count closure: %v", err)
	}
	return count
}

func countGroupAccessRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&AdminGroupAccess{}).Count(&count).Error; err != nil {
		t.Fatalf("count group access: %v", err)
	}
	return count
}
