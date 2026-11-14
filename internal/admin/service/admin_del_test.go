package service

import (
	"testing"

	cErr "buildadmin-go/internal/pkg/error"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAdminIDs(t *testing.T) {
	t.Parallel()
	got, err := normalizeIDs([]int32{4, 4, 2, 4}, "admin", false, func(_ int32) error {
		return cErr.BadRequest("ids must be positive")
	})
	require.NoError(t, err)
	require.Equal(t, []int32{4, 2}, got)
	for _, ids := range [][]int32{{0}, {-1}, {2, 0, 3}} {
		_, err := normalizeIDs(ids, "admin", false, func(_ int32) error {
			return cErr.BadRequest("ids must be positive")
		})
		require.Error(t, err, "normalizeIDs(%v) should reject non-positive IDs", ids)
	}
}
