package repository

import (
	"testing"

	"buildadmin-go/internal/pkg/data_scope"

	"github.com/stretchr/testify/require"
)

func TestFrameworkAdminOwnerTablesRegistered(t *testing.T) {
	tables := data_scope.AdminOwnerTables()
	require.Contains(t, tables, "user")
	require.Contains(t, tables, "user_money_log")
}
