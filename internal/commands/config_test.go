package commands

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestApplyGinMode 覆盖 app.env → gin 运行模式的映射。gin mode 是包级状态，
// 测试通过 t.Cleanup 恢复原值，避免污染同包其它测试。
func TestApplyGinMode(t *testing.T) {
	original := gin.Mode()
	t.Cleanup(func() { gin.SetMode(original) })

	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "debug maps to gin debug", env: "debug", want: gin.DebugMode},
		{name: "release maps to gin release", env: "release", want: gin.ReleaseMode},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := applyGinMode(test.env); err != nil {
				t.Fatalf("applyGinMode(%q) error = %v", test.env, err)
			}
			if got := gin.Mode(); got != test.want {
				t.Fatalf("applyGinMode(%q) set mode %q, want %q", test.env, got, test.want)
			}
		})
	}

	t.Run("invalid env errors and leaves mode unchanged", func(t *testing.T) {
		before := gin.Mode()
		err := applyGinMode("local")
		if err == nil {
			t.Fatal("applyGinMode(\"local\") error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "local") {
			t.Fatalf("applyGinMode(\"local\") error = %q, want it to name the offending value", err)
		}
		if after := gin.Mode(); after != before {
			t.Fatalf("mode changed from %q to %q on error", before, after)
		}
	})
}
