package random

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	md5Pattern  = regexp.MustCompile(`^[0-9a-f]{32}$`)
	sha1Pattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func TestBuildFromPools(t *testing.T) {
	t.Parallel()

	pools := map[string]string{
		"alpha":   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"alnum":   "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"numeric": "0123456789",
		"noZero":  "123456789",
	}
	// 长度跨越 0、小于池长、等于池长、大于池长等边界。
	lengths := []int{0, 1, 8, 32, 64, 200}

	for _, typ := range []string{"alpha", "alnum", "numeric", "noZero"} {
		for _, length := range lengths {
			t.Run(fmt.Sprintf("%s_%d", typ, length), func(t *testing.T) {
				t.Parallel()

				got := Build(typ, length)
				require.Len(t, got, length)
				for i := 0; i < len(got); i++ {
					require.Truef(t, strings.ContainsRune(pools[typ], rune(got[i])),
						"Build(%q, %d) 产生池外字符 %q", typ, length, string(got[i]))
				}
			})
		}
	}
}

func TestBuildUnknownTypeReturnsInput(t *testing.T) {
	t.Parallel()

	for _, typ := range []string{"", "unknown", "unique", "encrypt"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, typ, Build(typ, 16))
		})
	}
}

func TestBuildMD5(t *testing.T) {
	t.Parallel()

	first := Build("md5", 0)
	second := Build("md5", 0)
	require.Regexp(t, md5Pattern, first)
	require.Regexp(t, md5Pattern, second)
	require.NotEqual(t, first, second)
}

func TestBuildSHA1(t *testing.T) {
	t.Parallel()

	first := Build("sha1", 0)
	second := Build("sha1", 0)
	require.Regexp(t, sha1Pattern, first)
	require.Regexp(t, sha1Pattern, second)
	require.NotEqual(t, first, second)
}

func TestUuidShape(t *testing.T) {
	t.Parallel()

	first := Uuid()
	second := Uuid()
	require.Regexp(t, uuidPattern, first)
	require.Regexp(t, uuidPattern, second)
	require.NotEqual(t, first, second)
}

func TestSecureIntnRange(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 10, 1000} {
		for i := 0; i < 200; i++ {
			v := SecureIntn(n)
			require.GreaterOrEqual(t, v, 0)
			require.Less(t, v, n)
		}
	}
}

func TestSecureIntnPanicsOnNonPositive(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() { SecureIntn(0) })
	require.Panics(t, func() { SecureIntn(-1) })
}
