package user

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
