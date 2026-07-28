package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-build-admin/utils"
)

func TestFrameworkVersionFileMatchesConstant(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(utils.RootPath(), "FRAMEWORK_VERSION"))
	if err != nil {
		t.Fatalf("read FRAMEWORK_VERSION: %v", err)
	}

	if got := strings.TrimSpace(string(data)); got != Framework {
		t.Fatalf("FRAMEWORK_VERSION = %q, want %q", got, Framework)
	}
}
