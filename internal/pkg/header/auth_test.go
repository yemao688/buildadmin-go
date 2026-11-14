package header

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestContext(t *testing.T) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	return c
}

func TestAdminAuthRoundtrip(t *testing.T) {
	t.Parallel()

	c := newTestContext(t)
	want := AdminAuth{
		Version:      "2.3.8",
		Language:     "zh-cn",
		IsLogin:      true,
		Id:           7,
		Username:     "admin",
		Token:        "token-value",
		IsSuperAdmin: true,
	}

	SetAdminAuth(c, want)
	require.Equal(t, want, GetAdminAuth(c))
}

func TestUserAuthRoundtrip(t *testing.T) {
	t.Parallel()

	c := newTestContext(t)
	want := UserAuth{
		Version:  "2.3.8",
		Language: "en",
		IsLogin:  true,
		Id:       42,
		Token:    "user-token",
	}

	SetUserAuth(c, want)
	require.Equal(t, want, GetUserAuth(c))
}

func TestAuthUnsetReturnsZeroValue(t *testing.T) {
	t.Parallel()

	c := newTestContext(t)
	require.Equal(t, AdminAuth{}, GetAdminAuth(c))
	require.Equal(t, UserAuth{}, GetUserAuth(c))
}

func TestAdminAndUserAuthIsolated(t *testing.T) {
	t.Parallel()

	c := newTestContext(t)
	adminAuth := AdminAuth{IsLogin: true, Id: 1, Token: "admin-token"}
	userAuth := UserAuth{IsLogin: true, Id: 2, Token: "user-token"}

	SetAdminAuth(c, adminAuth)
	require.Equal(t, UserAuth{}, GetUserAuth(c))

	SetUserAuth(c, userAuth)
	require.Equal(t, adminAuth, GetAdminAuth(c))
	require.Equal(t, userAuth, GetUserAuth(c))
}

func TestAuthWrongStoredTypePanics(t *testing.T) {
	t.Parallel()

	t.Run("admin", func(t *testing.T) {
		t.Parallel()

		c := newTestContext(t)
		c.Set(adminAuthKey, "not-an-admin-auth")
		require.Panics(t, func() { GetAdminAuth(c) })
	})

	t.Run("user", func(t *testing.T) {
		t.Parallel()

		c := newTestContext(t)
		c.Set(userAuthKey, 123)
		require.Panics(t, func() { GetUserAuth(c) })
	})
}
