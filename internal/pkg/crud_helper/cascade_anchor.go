package crud_helper

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
)

// 级联注册锚点：reassignable 主实体 repo 生成的 CascadeOwners() 方法体内
// 以这两行注释为界，生成器在子表生成/删除时自动维护其中的注册条目。锚点
// 标记各独占一行，中间初始为空。
const (
	cascadeAnchorBegin = "// @cascade:begin"
	cascadeAnchorEnd   = "// @cascade:end"
)

// cascadeAnchorEntry 生成锚点块内的一行注册条目。固定格式，OwnerColumn 恒为
// admin_id（级联硬契约：归属列必须为 admin_id）。
func cascadeAnchorEntry(table, byColumn string) string {
	return "\t\t{Table: \"" + table + "\", ByColumn: \"" + byColumn + "\", OwnerColumn: \"admin_id\"},"
}

// repoRootOverride 是仅供测试注入的仓库根覆盖（空值 = 默认 util.RootPath()）。
// 下游业务 fork 拥有自己的业务表 repo（如 seller_user.go）时，依赖"真实仓库
// 无该表 repo"的测试会失真的——用临时根隔离真实仓库状态，使 repo 落点检查
// 在隔离根下成立。生产代码路径不设置它。
var repoRootOverride string

// parentRepositoryPath 推导主实体 repo 文件路径（拍平布局
// internal/admin/repository/<table>.go），经既有路径解析与绝对路径归属校验
// 防注入路径。测试注入 repoRootOverride 时直接按覆盖根拼接（跳过解析校验，
// 仅供测试使用）。
func parentRepositoryPath(table string) (string, error) {
	if repoRootOverride != "" {
		return filepath.Join(repoRootOverride, filepath.FromSlash("internal/admin/repository"), table+".go"), nil
	}
	info, err := ParseRepositoryNameData(table, "")
	if err != nil {
		return "", err
	}
	if err := ValidateGeneratedAbsolutePath(info.ParseFile, "internal/admin/repository"); err != nil {
		return "", err
	}
	return info.ParseFile, nil
}

// findCascadeAnchor 定位锚点起止行号。begin/end 均为 -1 表示缺失；ok 仅在
// 两者都存在且 begin 在 end 之前时为真。
func findCascadeAnchor(content string) (begin, end int, ok bool) {
	begin, end = -1, -1
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == cascadeAnchorBegin {
			begin = i
		}
		if trimmed == cascadeAnchorEnd {
			end = i
		}
	}
	return begin, end, begin >= 0 && end >= 0 && begin < end
}

// DefaultSpecsDir 返回 crud_specs/ 目录（仓库根；镜像内随二进制分发）。
func DefaultSpecsDir() string {
	return filepath.Join(util.RootPath(), "crud_specs")
}

// validateInheritParent 校验 inheritFrom 指向的主实体具备可注册的级联父表
// 条件，并落实级联硬契约。事实源是 specsDir 目录里的父表 spec（声明式，
// 随仓库/镜像分发，重装不丢），不再依赖 crud_log 生成历史：
//   - 父表 spec 必须存在（registerOnly 登记表或普通业务表均可）；
//   - 父表必须声明 reassignable: true（其 CascadeOwners() 锚点块才可注册）；
//   - 硬契约：主实体归属列必须为 admin_id（auto 模式 ownerColumn 为空视为
//     admin_id；required 显式声明时必须是 admin_id）；
//   - 硬契约：主实体主键必须为 id（模板 `Where("id = ?", ...)` 与
//     cascade:sync 的 JOIN `p.id` 按此拼接；非 id 主键运行时必炸）；
//   - 父表 repo 文件必须已存在且含 CascadeOwners() 锚点块（registerOnly 表
//     手写 repo 自带，业务表由生成器渲染）——applyCascadeAnchor 的落点。
func validateInheritParent(specsDir, parentTable string) error {
	parentSpec, err := loadSpecByTable(specsDir, parentTable)
	if err != nil {
		return fmt.Errorf("inheritFrom parent %q spec not found in %s: %w; add %s/%s.yaml with dataScope.reassignable=true first", parentTable, specsDir, err, specsDir, parentTable)
	}
	ds := parentSpec.Table.DataScope
	if ds == nil || !ds.Reassignable {
		return fmt.Errorf("inheritFrom parent %q spec %s does not declare dataScope.reassignable=true; add or fix it first", parentTable, specFileNameFor(specsDir, parentTable))
	}
	// 硬契约：主实体归属列必须是 admin_id（auto 模式 ownerColumn 为空，视为
	// 解析为 admin_id；required 显式声明时必须是 admin_id）。
	if owner := ds.OwnerColumn; owner != "" && owner != "admin_id" {
		return fmt.Errorf("inheritFrom parent %q declares owner column %q; cascade contract requires admin_id", parentTable, owner)
	}
	// 硬契约：主实体主键必须为 id（模板 `Where("id = ?", ...)` 与
	// cascade:sync 的 JOIN `p.id` 按此拼接；非 id 主键运行时必炸）。
	pks := specPrimaryKeys(parentSpec.Fields)
	if len(pks) != 1 || pks[0] != "id" {
		return fmt.Errorf("inheritFrom parent %q primary key must be exactly id (cascade contract), got %v", parentTable, pks)
	}
	// 锚点落点：父表 repo 必须存在且含 CascadeOwners() 锚点块（registerOnly
	// 表手写 repo 自带；业务表生成器渲染）。文件系统检查替代 crud_log 的
	// "已成功生成"证据——比生成历史更直接可靠。
	repoPath, err := parentRepositoryPath(parentTable)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(repoPath)
	if err != nil {
		return fmt.Errorf("inheritFrom parent %q repository %s not found: %w; generate or hand-write it with a CascadeOwners() anchor block first", parentTable, repoPath, err)
	}
	if _, _, ok := findCascadeAnchor(string(content)); !ok {
		return fmt.Errorf("inheritFrom parent %q repository %s has no cascade anchor block (@cascade:begin/end); add it first", parentTable, repoPath)
	}
	return nil
}

// loadSpecByTable 从 specsDir 目录按逻辑表名加载 spec（文件名须为
// <table>.yaml，与 crud:generate 的调用约定一致）。目录缺失视为无此 spec。
func loadSpecByTable(specsDir, table string) (*GenerateOptions, error) {
	path := specFileNameFor(specsDir, table)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return LoadSpec(path)
}

// specFileNameFor 返回逻辑表名对应的 specs 目录文件路径。
func specFileNameFor(specsDir, table string) string {
	return filepath.Join(specsDir, table+".yaml")
}

// reapplyCascadeAnchors 在主实体重新生成后调用：重新生成会把 CascadeOwners()
// 渲染为空锚点块（重置窗口），此处按 crud_specs/ 中仍声明 inheritFrom 指向该
// 主实体的子表逐条重注册（applyCascadeAnchor 整行幂等），消除重置窗口内的
// 静默级联失效。无引用时为空操作。声明事实源是 specs 目录（非 crud_log）。
func reapplyCascadeAnchors(specsDir, parentTable string) error {
	referrers, err := findInheritReferrers(specsDir, parentTable)
	if err != nil {
		return err
	}
	if len(referrers) == 0 {
		return nil
	}
	repoPath, err := parentRepositoryPath(parentTable)
	if err != nil {
		return err
	}
	for _, child := range referrers {
		childSpec, err := loadSpecByTable(specsDir, child)
		if err != nil {
			return fmt.Errorf("reapply cascade anchor for child %q: %w", child, err)
		}
		inherit := inheritRefOf(childSpec.Table.DataScope)
		if inherit == nil || inherit.Table != parentTable {
			continue
		}
		if err := applyCascadeAnchor(repoPath, child, inherit.ByColumn); err != nil {
			return fmt.Errorf("reapply cascade anchor for child %q: %w", child, err)
		}
	}
	return nil
}

// inheritRefOf 从 data-scope 配置中取出 inheritFrom 声明（nil 安全）。
func inheritRefOf(cfg *data_scope.Config) *data_scope.InheritRef {
	if cfg == nil {
		return nil
	}
	return cfg.InheritFrom
}

// InheritDeclaration 是 crud_specs/ 中一个子表的级联声明：子表名、父表名与
// 关联列。specs 是级联声明的唯一事实源（随仓库/镜像分发，重装不丢）。
type InheritDeclaration struct {
	ChildTable  string
	ParentTable string
	ByColumn    string
}

// ScanInheritDeclarations 扫描 specsDir 目录的 *.yaml，返回全部 inheritFrom
// 声明（确定性排序）。被 cascade:sync 聚合与 findInheritReferrers 引用检查
// 共用。spec 无法解析（LoadSpec 失败）时 fail-closed 报错：声明损坏必须修复，
// 对账宁可中断也不静默漏掉潜在脏数据。
func ScanInheritDeclarations(specsDir string) ([]InheritDeclaration, error) {
	entries, err := filepath.Glob(filepath.Join(specsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("scan spec dir %q: %w", specsDir, err)
	}
	sort.Strings(entries)
	decls := make([]InheritDeclaration, 0)
	for _, path := range entries {
		spec, err := LoadSpec(path)
		if err != nil {
			return nil, fmt.Errorf("load spec %q: %w", path, err)
		}
		inherit := inheritRefOf(spec.Table.DataScope)
		if inherit == nil || inherit.Table == "" || inherit.ByColumn == "" {
			continue
		}
		decls = append(decls, InheritDeclaration{
			ChildTable:  spec.Table.Name,
			ParentTable: inherit.Table,
			ByColumn:    inherit.ByColumn,
		})
	}
	return decls, nil
}

// findInheritReferrers 扫描 specsDir 中声明 inheritFrom 指向 parentTable 的
// 子表名列表（确定性排序）。主实体自身的 spec 不计入引用。
func findInheritReferrers(specsDir, parentTable string) ([]string, error) {
	decls, err := ScanInheritDeclarations(specsDir)
	if err != nil {
		return nil, err
	}
	var referrers []string
	for _, decl := range decls {
		if decl.ChildTable == parentTable {
			continue
		}
		if decl.ParentTable == parentTable {
			referrers = append(referrers, decl.ChildTable)
		}
	}
	sort.Strings(referrers)
	return referrers, nil
}

// cleanupStaleCascadeAnchors 在子表生成/重新生成前清理旧的级联锚点注册：
// 扫描 specsDir 中全部声明 reassignable 的父表 repo 锚点块，凡含 childTable
// 条目的（无论其 spec 当前是否仍指向该父表）一律移除——子表已独立或改指向时
// 旧父表锚点里的注册条目不再级联它。返回实际被清理的父表 repo 路径列表（调用
// 方纳入快照，失败可回滚）。事实源是 specs 目录与 repo 锚点块本身，不依赖
// crud_log 生成历史。
func cleanupStaleCascadeAnchors(specsDir, childTable string) ([]string, error) {
	entries, err := filepath.Glob(filepath.Join(specsDir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("scan spec dir %q: %w", specsDir, err)
	}
	sort.Strings(entries)
	cleaned := make([]string, 0)
	for _, path := range entries {
		spec, err := LoadSpec(path)
		if err != nil {
			return nil, fmt.Errorf("load spec %q: %w", path, err)
		}
		ds := spec.Table.DataScope
		if ds == nil || !ds.Reassignable {
			continue
		}
		repoPath, err := parentRepositoryPath(spec.Table.Name)
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(repoPath)
		if err != nil {
			// 父表 repo 缺失：无锚点可清理，跳过（父表未生成时没有级联注册）。
			continue
		}
		if _, _, ok := findCascadeAnchor(string(content)); !ok {
			continue
		}
		if err := removeCascadeAnchor(repoPath, childTable); err != nil {
			return nil, fmt.Errorf("cleanup stale cascade anchor for child %q from parent %q: %w", childTable, spec.Table.Name, err)
		}
		cleaned = append(cleaned, repoPath)
	}
	return cleaned, nil
}

// cascadeAnchorEntryPattern 匹配锚点块内指向指定子表的一行注册条目。空白容忍
// （gofmt/手改的空格差异不破坏幂等与替换判定）；表名经 regexp.QuoteMeta 转义，
// 表名尾部引号限定整词（child 不误伤 child2）。
func cascadeAnchorEntryPattern(table string) *regexp.Regexp {
	return regexp.MustCompile(`\{\s*Table\s*:\s*"` + regexp.QuoteMeta(table) + `"\s*,`)
}

// applyCascadeAnchor 向主实体 repo 的锚点块写入一条子表注册条目。严格要求：
// 文件必须存在且含完整锚点块，否则报错（子表生成前主实体必须先以
// reassignable 生成）。幂等与替换按整行匹配（空白容忍）：
//   - 锚点块内已存在完全相同的整行 → 跳过写入（幂等）；
//   - 存在同 Table 的整行（含 ByColumn 不同或手改过格式的）→ 原位替换为新
//     条目，且同 Table 的重复条目全部收敛为一条（子表重新生成改 byColumn 时
//     不残留旧列，避免级联按旧列 UPDATE 静默漂移；手改残留的重复行也会收敛）；
//   - 否则在 `// @cascade:end` 之前插入。
func applyCascadeAnchor(repoFilePath, table, byColumn string) error {
	content, err := os.ReadFile(repoFilePath)
	if err != nil {
		return fmt.Errorf("apply cascade anchor: read parent repository %q: %w", repoFilePath, err)
	}
	begin, end, ok := findCascadeAnchor(string(content))
	if !ok {
		return fmt.Errorf("apply cascade anchor: anchor block not found in %q; regenerate the parent table with a reassignable dataScope first", repoFilePath)
	}
	entry := cascadeAnchorEntry(table, byColumn)
	pattern := cascadeAnchorEntryPattern(table)
	lines := strings.Split(string(content), "\n")
	// 单遍扫描：锚点块内同 Table 的行收敛为一条规范条目，其余行原样保留。
	// 精确条目存在时也继续扫描（收敛手改残留的重复行），仅在内容变化时写回。
	// 未命中任何同 Table 行时，在 `// @cascade:end`（下标 end）之前插入。
	kept := make([]string, 0, len(lines)+1)
	seeded := false // 是否已在块内放置规范条目
	changed := false
	for i, line := range lines {
		if i == end && !seeded {
			kept = append(kept, entry)
			seeded = true
			changed = true
		}
		if i <= begin || i >= end {
			kept = append(kept, line)
			continue
		}
		if line == entry {
			if !seeded {
				kept = append(kept, line)
				seeded = true
			} else {
				changed = true // 重复的精确条目：收敛
			}
			continue
		}
		if pattern.MatchString(line) {
			if !seeded {
				kept = append(kept, entry)
				seeded = true
			}
			changed = true // 变体/旧 ByColumn 行：替换或收敛
			continue
		}
		kept = append(kept, line)
	}
	if !changed {
		return nil
	}
	return writePreservingMode(repoFilePath, strings.Join(kept, "\n"))
}

// removeCascadeAnchor 从主实体 repo 的锚点块移除子表注册条目。幂等容忍：
// 文件不存在或锚点缺失直接返回 nil（防止 crud:delete 因主实体已删/锚点
// 结构漂移而失败）；条目不存在同样直接返回 nil。
func removeCascadeAnchor(repoFilePath, table string) error {
	content, err := os.ReadFile(repoFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	begin, end, ok := findCascadeAnchor(string(content))
	if !ok {
		return nil
	}
	pattern := cascadeAnchorEntryPattern(table)
	lines := strings.Split(string(content), "\n")
	updated := make([]string, 0, len(lines))
	removed := false
	for i, line := range lines {
		if i > begin && i < end && pattern.MatchString(line) {
			removed = true
			continue
		}
		updated = append(updated, line)
	}
	if !removed {
		return nil
	}
	return writePreservingMode(repoFilePath, strings.Join(updated, "\n"))
}

// writePreservingMode 原子写回：同目录临时文件 + rename，崩溃不留下截断的
// 主实体 repo（评审 MINOR：os.WriteFile 原地截断，中断会破坏文件并使后续
// 生成失败）。
func writePreservingMode(path, content string) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
