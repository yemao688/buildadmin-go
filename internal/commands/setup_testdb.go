package commands

import (
	"fmt"
	"strconv"
	"strings"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/installer"
)

const setupTestDBCharset = "utf8mb4"

// mergeMySQLTestConfig 把 mysql_test 段合并进稀疏覆盖层 configs/config.yaml：
// 读现有 YAML、只改这一段、写回，其它既有键（mysql、token.key、app.env 等）原样
// 保留。写入沿用 WriteConfigOverrides 的文件头与 0600 权限，与安装器其它写入一致。
func mergeMySQLTestConfig(path string, section map[string]any) error {
	if len(section) == 0 {
		return nil
	}
	return conf.WriteConfigOverrides(path, map[string]any{"mysql_test": section})
}

// setupMySQLTestSection 按 config.defaults.yaml 的 mysql_test 键名构造覆盖段，
// 测试库复用主库的连接参数（host/port/username/password）。
func setupMySQLTestSection(database installer.Database, testDBName string) (map[string]any, error) {
	port, err := strconv.Atoi(database.Hostport)
	if err != nil {
		return nil, fmt.Errorf("MySQL port 无效: %q", database.Hostport)
	}
	return map[string]any{
		"enabled":  true,
		"host":     database.Hostname,
		"port":     port,
		"database": testDBName,
		"username": database.Username,
		"password": database.Password,
		"charset":  setupTestDBCharset,
	}, nil
}

// setupTestDatabaseName 返回测试库名：--test-db-name 优先，缺省 <主库名>_test；
// recovery 测试再在该名字上派生 _fresh_* 库。
func setupTestDatabaseName(options setupOptions, database installer.Database) string {
	if options.testDBName != "" {
		return options.testDBName
	}
	return database.Database + "_test"
}

// prepareSetupTestDatabase 在连接配置落盘后按需准备 MySQL 集成测试库：建库、
// 授予 `<主库>%` 通配权限、把 mysql_test 段合并进 configs/config.yaml。
// 测试库是辅助资源，任何一步失败都只打印手工指引并继续安装；建库失败时不写
// mysql_test 段，避免留下指向不存在库的 enabled:true 配置。
func (r setupRunner) prepareSetupTestDatabase(options setupOptions, input setupInput, console *setupConsole) {
	prepare := options.withTestDB
	if !prepare && !options.yes {
		answer, err := console.line(fmt.Sprintf("是否顺带准备 MySQL 测试库 %s？[y/N] ", setupTestDatabaseName(options, input.database)), "")
		if err != nil || !isSetupYes(answer) {
			return
		}
		prepare = true
	}
	if !prepare {
		return
	}

	name := setupTestDatabaseName(options, input.database)
	if err := r.deps.createTestDatabase(input.database, name); err != nil {
		_, _ = fmt.Fprintf(r.out, "创建测试库 %s 失败: %v\n", name, err)
		_, _ = fmt.Fprintln(r.out, "测试库配置已跳过；可手工执行以下 SQL 后重新安装（或加 --with-test-db 重跑 setup）：")
		r.printTestDatabaseSQL(input.database, name)
		return
	}

	grantErr := r.deps.grantTestDatabase(input.database, input.database.Database)
	section, err := setupMySQLTestSection(input.database, name)
	if err != nil {
		_, _ = fmt.Fprintf(r.out, "跳过 mysql_test 配置: %v\n", err)
		return
	}
	if err := r.deps.mergeTestConfig(r.configPath, section); err != nil {
		_, _ = fmt.Fprintf(r.out, "写入 configs/config.yaml 的 mysql_test 段失败: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(r.out, "测试库 %s 已就绪，已写入 configs/config.yaml 的 mysql_test 段（enabled: true）\n", name)
	if grantErr != nil {
		_, _ = fmt.Fprintf(r.out, "授予通配权限失败: %v\n", grantErr)
		_, _ = fmt.Fprintln(r.out, "普通集成测试不受影响；recovery 等会派生独立库的测试需要管理员账号手工执行：")
		r.printTestDatabaseSQL(input.database, name)
	}
}

// printTestDatabaseSQL 输出可直接执行的手工建库与授权语句（不含密码）。
func (r setupRunner) printTestDatabaseSQL(database installer.Database, testDBName string) {
	identifier := func(value string) string { return "`" + strings.ReplaceAll(value, "`", "``") + "`" }
	account := "'" + strings.ReplaceAll(database.Username, "'", "''") + "'@'%'"
	_, _ = fmt.Fprintf(r.out, "  CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n", identifier(testDBName))
	_, _ = fmt.Fprintf(r.out, "  GRANT ALL PRIVILEGES ON %s.* TO %s;\n", identifier(database.Database+"%"), account)
}
