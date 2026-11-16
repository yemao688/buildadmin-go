package installer

import (
	"database/sql"
	"fmt"
	"strings"
)

// CreateTestDatabase 用安装连接参数（同 host/port/账号）创建 MySQL 集成测试库；
// 库已存在时保持原样，新建时字符集与主库一致（utf8mb4）。
func CreateTestDatabase(cfg Database, name string) error {
	db, err := sql.Open("mysql", serverDSN(cfg))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// 库名来自安装参数，一律反引号转义后拼接；CREATE DATABASE 不接受占位参数。
	statement := "CREATE DATABASE IF NOT EXISTS " + quoteSQLIdentifier(name) +
		" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	_, err = db.Exec(statement)
	return err
}

// GrantDatabaseWildcard 授予安装账号在 `<database>%` 上的数据库级全部权限：
// recovery 等测试会派生 `<database>_fresh_*` 库，只授权 `<database>_test` 不够。
// 该语句要求账号带 GRANT OPTION，安装账号通常没有，因此调用方必须把错误降级
// 为手工指引，而不是中断安装。
func GrantDatabaseWildcard(cfg Database, database string) error {
	db, err := sql.Open("mysql", serverDSN(cfg))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	statement := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s",
		quoteSQLIdentifier(database+"%"), quoteSQLAccount(cfg.Username))
	_, err = db.Exec(statement)
	return err
}

// quoteSQLIdentifier 反引号转义 MySQL 标识符（含库名通配模式里的 %/_）。
func quoteSQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// quoteSQLAccount 把用户名格式化为 '<user>'@'%'，单引号按 MySQL 规则双写转义。
func quoteSQLAccount(username string) string {
	return "'" + strings.ReplaceAll(username, "'", "''") + "'@'%'"
}
