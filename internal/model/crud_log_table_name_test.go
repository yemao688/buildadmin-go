package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// 回归测试：Log struct 经 GORM 命名策略推导的表名必须指向真实 crud_log 表。
func TestLogStructTableName(t *testing.T) {
	namer := schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"}
	s, err := schema.Parse(Log{}, &sync.Map{}, namer)
	require.NoError(t, err)
	require.Equal(t, "ba_crud_log", s.Table)
}
