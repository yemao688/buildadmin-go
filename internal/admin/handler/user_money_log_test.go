package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMoneyHandlerBindsDecimalAmount(t *testing.T) {
	var request Money
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":1.25,"memo":"x"}`), &request))
	amount, err := parseMoneyAmount(request.Money)
	require.NoError(t, err)
	require.Equal(t, float64(1.25), amount)

	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25","memo":"x"}`), &request))
	amount, err = parseMoneyAmount(request.Money)
	require.NoError(t, err)
	require.Equal(t, float64(1.25), amount)
}

func TestMoneyHandlerRejectsMoreThanTwoDecimals(t *testing.T) {
	var request Money
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.255","memo":"x"}`), &request))
	_, err := parseMoneyAmount(request.Money)
	require.Error(t, err)
}

func TestMoneyLogTypeFor(t *testing.T) {
	// 超管可指定任意类型（含空 → 交给 money 链路的缺省 system）。
	require.Equal(t, "recharge", moneyLogTypeFor("recharge", true))
	require.Equal(t, "withdraw", moneyLogTypeFor("withdraw", true))
	require.Equal(t, "", moneyLogTypeFor("", true))
	// 非超管一律强制 system，忽略请求值。
	require.Equal(t, "system", moneyLogTypeFor("recharge", false))
	require.Equal(t, "system", moneyLogTypeFor("extend", false))
	require.Equal(t, "system", moneyLogTypeFor("", false))
}

func TestMoneyMemoIsOptional(t *testing.T) {
	// 备注不再必填：缺省 memo 应能正常绑定（无 binding:"required"）。
	var request Money
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25"}`), &request))
	require.Equal(t, "", request.Memo)
	// 仍接受带备注的请求。
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25","memo":"x"}`), &request))
	require.Equal(t, "x", request.Memo)
}
