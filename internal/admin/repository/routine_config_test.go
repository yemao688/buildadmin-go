package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 变量名/用户名唯一性预检已上移 service；repo 只保留并发插入的兜底映射，
// 其判据必须同时识别 GORM 翻译后的哨兵与 MySQL 原生 1062。
func TestIsDuplicateKeyErrorRecognizesUniqueViolations(t *testing.T) {
	t.Parallel()
	require.True(t, isDuplicateKeyError(gorm.ErrDuplicatedKey))
	require.True(t, isDuplicateKeyError(fmt.Errorf("create: %w", gorm.ErrDuplicatedKey)))
	require.True(t, isDuplicateKeyError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}))
	require.False(t, isDuplicateKeyError(gorm.ErrRecordNotFound))
	require.False(t, isDuplicateKeyError(&mysql.MySQLError{Number: 1064, Message: "syntax"}))
	require.False(t, isDuplicateKeyError(errors.New("plain failure")))
}
