package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserMoneyLogParamAmountFormats(t *testing.T) {
	var request UserMoneyLogParam
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":1.25,"memo":"x"}`), &request))
	amount, err := request.Amount()
	require.NoError(t, err)
	require.Equal(t, float64(1.25), amount)

	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25","memo":"x"}`), &request))
	amount, err = request.Amount()
	require.NoError(t, err)
	require.Equal(t, float64(1.25), amount)

	// 负数与零沿用历史语义：允许，不做正数限制。
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"-2.5"}`), &request))
	amount, err = request.Amount()
	require.NoError(t, err)
	require.Equal(t, float64(-2.5), amount)

	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":0}`), &request))
	amount, err = request.Amount()
	require.NoError(t, err)
	require.Zero(t, amount)
}

func TestUserMoneyLogParamAmountRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "three decimals", body: `{"user_id":1,"money":"1.255"}`, want: "money must have at most two decimals"},
		{name: "exponent", body: `{"user_id":1,"money":"1e5"}`, want: "invalid money amount"},
		{name: "unsigned exponent", body: `{"user_id":1,"money":"1E5"}`, want: "invalid money amount"},
		{name: "plus sign", body: `{"user_id":1,"money":"+1"}`, want: "invalid money amount"},
		{name: "nan text", body: `{"user_id":1,"money":"NaN"}`, want: "invalid money amount"},
		{name: "inf text", body: `{"user_id":1,"money":"Inf"}`, want: "invalid money amount"},
		{name: "empty string", body: `{"user_id":1,"money":""}`, want: "invalid money amount"},
		{name: "null", body: `{"user_id":1,"money":null}`, want: "invalid money amount"},
		{name: "letters", body: `{"user_id":1,"money":"abc"}`, want: "invalid money amount"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var request UserMoneyLogParam
			require.NoError(t, json.Unmarshal([]byte(tc.body), &request))
			_, err := request.Amount()
			require.EqualError(t, err, tc.want)
		})
	}
}

func TestUserMoneyLogParamMemoIsOptional(t *testing.T) {
	// 备注可选：缺省 memo 应能正常绑定（无 binding:"required"）。
	var request UserMoneyLogParam
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25"}`), &request))
	require.Equal(t, "", request.Memo)
	// 仍接受带备注的请求。
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25","memo":"x"}`), &request))
	require.Equal(t, "x", request.Memo)
}
