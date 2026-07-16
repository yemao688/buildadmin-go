package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/safeint"
)

func TestMoneyHandlerBindsDecimalAmount(t *testing.T) {
	var request Money
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":1.25,"memo":"x"}`), &request))
	cents, err := safeint.ParseDecimalCents(request.Money)
	require.NoError(t, err)
	require.Equal(t, int32(125), cents)

	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.25","memo":"x"}`), &request))
	cents, err = safeint.ParseDecimalCents(request.Money)
	require.NoError(t, err)
	require.Equal(t, int32(125), cents)
}

func TestMoneyHandlerRejectsMoreThanTwoDecimals(t *testing.T) {
	var request Money
	require.NoError(t, json.Unmarshal([]byte(`{"user_id":1,"money":"1.255","memo":"x"}`), &request))
	_, err := safeint.ParseDecimalCents(request.Money)
	require.Error(t, err)
}
