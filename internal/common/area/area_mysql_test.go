package area

import (
	"context"
	"testing"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// newAreaFixture 在独立 fixture 库建 area 表并写入固定的省/市/区县层级
// 数据，供逐级下钻断言使用。
func newAreaFixture(t *testing.T) *AreaModel {
	t.Helper()
	db, cfg := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	require.NoError(t, db.AutoMigrate(&Area{}))
	require.NoError(t, db.Create(&[]Area{
		{ID: 1, Pid: 0, Name: "北京市", Level: 1},
		{ID: 2, Pid: 0, Name: "上海市", Level: 1},
		{ID: 11, Pid: 1, Name: "北京城区", Level: 2},
		{ID: 21, Pid: 2, Name: "上海城区", Level: 2},
		{ID: 111, Pid: 11, Name: "东城区", Level: 3},
		{ID: 112, Pid: 11, Name: "西城区", Level: 3},
		{ID: 211, Pid: 21, Name: "黄浦区", Level: 3},
	}).Error)
	return NewAreaModel(db, cfg)
}

// areaOptions 把 List 返回的匿名结构体切片转成 label→value 映射，避免断言
// 依赖 MySQL 未显式 ORDER BY 时的行序。
func areaOptions(t *testing.T, got any) map[string]int32 {
	t.Helper()
	options, ok := got.([]struct {
		Value int32  `json:"value"`
		Label string `json:"label"`
	})
	require.True(t, ok, "List 返回类型异常: %T", got)
	result := make(map[string]int32, len(options))
	for _, option := range options {
		result[option.Label] = option.Value
	}
	return result
}

func TestAreaListByLevel(t *testing.T) {
	t.Parallel()
	am := newAreaFixture(t)

	cases := []struct {
		intent   string
		province string
		city     string
		want     map[string]int32
	}{
		{"省级：空 province 取 level=1", "", "", map[string]int32{"北京市": 1, "上海市": 2}},
		{"市级：province 下取 level=2", "2", "", map[string]int32{"上海城区": 21}},
		{"区县：province+city 下取 level=3", "1", "11", map[string]int32{"东城区": 111, "西城区": 112}},
		{"父级无匹配时返回空列表", "999", "", map[string]int32{}},
	}
	for _, tc := range cases {
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			got, err := am.List(context.Background(), tc.province, tc.city)
			require.NoError(t, err)
			require.Equal(t, tc.want, areaOptions(t, got))
		})
	}
}

func TestAreaListReturnsDBError(t *testing.T) {
	t.Parallel()
	db, cfg := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}

	// 不建 area 表：查询应把数据库错误原样返回，而不是吞成空列表。
	got, err := NewAreaModel(db, cfg).List(context.Background(), "", "")
	require.Error(t, err)
	require.Empty(t, got)
}
