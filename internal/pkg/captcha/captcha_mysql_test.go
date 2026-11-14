package captcha

import (
	"strings"
	"testing"
	"time"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
)

func TestCaptchaDatabaseFlow(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)
	require.NoError(t, db.AutoMigrate(&Captcha{}))

	svc := NewCaptchaService(db)

	rowCount := func(t *testing.T, id string) int64 {
		t.Helper()
		var n int64
		require.NoError(t, svc.sqlDB.Model(&Captcha{}).
			Where("`key`=?", svc.authCode(svc.config.SeKey, id, 32)).
			Count(&n).Error)
		return n
	}

	insertRow := func(t *testing.T, id, code string, expireAt int64) {
		t.Helper()
		require.NoError(t, svc.sqlDB.Model(&Captcha{}).Create(&Captcha{
			Key:        svc.authCode(svc.config.SeKey, id, 32),
			Code:       code,
			Captcha:    "ABCD",
			CreateTime: time.Now().Unix(),
			ExpireTime: expireAt,
		}).Error)
	}

	t.Run("create_and_get", func(t *testing.T) {
		t.Parallel()

		const id = "create-get"
		code, err := svc.Create(id)
		require.NoError(t, err)
		require.Len(t, code, 4)

		data, err := svc.GetCaptchaData(id)
		require.NoError(t, err)
		require.Equal(t, code, data.Captcha)
		require.Len(t, data.Code, LogicCaptcha)
		require.Equal(t, svc.authCode(svc.config.SeKey, id, 32), data.Key)
		require.Greater(t, data.ExpireTime, time.Now().Unix())
	})

	t.Run("check_empty_code", func(t *testing.T) {
		t.Parallel()

		const id = "empty-code"
		_, err := svc.Create(id)
		require.NoError(t, err)

		require.False(t, svc.Check("", id, LogicCaptcha))
		// 空验证码在读取记录前就返回，记录不应被删除。
		require.Equal(t, int64(1), rowCount(t, id))
	})

	t.Run("check_unknown_id", func(t *testing.T) {
		t.Parallel()

		require.False(t, svc.Check("ABCD", "no-such-id", LogicCaptcha))
	})

	t.Run("check_wrong_code", func(t *testing.T) {
		t.Parallel()

		const id = "wrong-code"
		_, err := svc.Create(id)
		require.NoError(t, err)

		// CodeSet 不含数字，0000 不可能与生成的验证码相同。
		require.False(t, svc.Check("0000", id, LogicCaptcha))
		require.Equal(t, int64(1), rowCount(t, id))
	})

	t.Run("check_correct_code_resets", func(t *testing.T) {
		t.Parallel()

		const id = "correct-code"
		code, err := svc.Create(id)
		require.NoError(t, err)

		// Check 内部统一转大写，小写输入同样通过。
		require.True(t, svc.Check(strings.ToLower(code), id, LogicCaptcha))
		require.Equal(t, int64(0), rowCount(t, id))
	})

	t.Run("check_type_mismatch", func(t *testing.T) {
		t.Parallel()

		const id = "type-mismatch"
		code, err := svc.Create(id)
		require.NoError(t, err)

		// 逻辑验证码密文长度 32，按图形验证码类型校验必须失败且不删除记录。
		require.False(t, svc.Check(code, id, GraphicCaptcha))
		require.Equal(t, int64(1), rowCount(t, id))
	})

	t.Run("check_expired", func(t *testing.T) {
		t.Parallel()

		const id = "expired"
		const code = "ABCD"
		insertRow(t, id, svc.authCode(code, id, LogicCaptcha), time.Now().Unix()-1)

		require.False(t, svc.Check(code, id, LogicCaptcha))
		require.Equal(t, int64(0), rowCount(t, id))
	})

	t.Run("check_code_length_mismatch", func(t *testing.T) {
		t.Parallel()

		const id = "length-mismatch"
		insertRow(t, id, "0123456789", time.Now().Unix()+60)

		require.False(t, svc.Check("ABCD", id, LogicCaptcha))
		require.Equal(t, int64(1), rowCount(t, id))
	})
}
