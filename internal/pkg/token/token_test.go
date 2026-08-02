package token

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type getForTestDriver struct {
	data *Token
	err  error
}

func (d getForTestDriver) Set(string, string, int32, int64) error { return nil }
func (d getForTestDriver) Get(string) (*Token, error)             { return d.data, d.err }
func (d getForTestDriver) Check(string, string, int32) bool       { return false }
func (d getForTestDriver) Delete(string) error                    { return nil }
func (d getForTestDriver) Clear(string, int32) error              { return nil }

func TestTokenHelperGetForChecksTokenType(t *testing.T) {
	tokenData := &Token{Token: "admin-token", Type: "admin", UserID: 7}
	helper := &TokenHelper{Driver: getForTestDriver{data: tokenData}}

	got, err := helper.GetFor("admin-token", "admin")
	require.NoError(t, err)
	require.Same(t, tokenData, got)

	_, err = helper.GetFor("admin-token", "user")
	require.EqualError(t, err, `token type mismatch: expected "user", got "admin"`)
}

func TestTokenHelperGetForPreservesGetError(t *testing.T) {
	getErr := errors.New("token store unavailable")
	helper := &TokenHelper{Driver: getForTestDriver{err: getErr}}

	_, err := helper.GetFor("token", "admin")
	require.ErrorIs(t, err, getErr)
}
