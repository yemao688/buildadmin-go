package user

import (
	"buildadmin-go/internal/model"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// 回归测试：去冗余前缀重命名后，struct 经 GORM 命名策略推导的表名
// 必须保持指向真实业务表（前缀安全），不得退化为实体名单词表。
func TestRenamedStructTableNames(t *testing.T) {
	namer := schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"}
	cases := map[string]any{
		"ba_user_money_log": model.MoneyLog{},
	}
	for want, model := range cases {
		s, err := schema.Parse(model, &sync.Map{}, namer)
		require.NoError(t, err)
		require.Equal(t, want, s.Table)
	}
}
