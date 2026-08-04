package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterRefreshTypeRejectsDuplicate(t *testing.T) {
	typ := "test-refresh-duplicate"
	desc := RefreshTypeDescriptor{AccessType: "test", AccessHeader: "test-token"}
	require.Error(t, RegisterRefreshType("", desc))
	require.NoError(t, RegisterRefreshType(typ, desc))
	require.Error(t, RegisterRefreshType(typ, desc))
	_, ok := LookupRefreshType(typ)
	require.True(t, ok)
	_, ok = LookupRefreshType("missing")
	require.False(t, ok)
}
