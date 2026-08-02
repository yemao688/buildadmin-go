package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/utils"
)

func TestFrameworkVersionFileMatchesConstant(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(utils.RootPath(), "VERSION_FRAMEWORK"))
	if err != nil {
		t.Fatalf("read VERSION_FRAMEWORK: %v", err)
	}

	if got := strings.TrimSpace(string(data)); got != Framework {
		t.Fatalf("VERSION_FRAMEWORK = %q, want %q", got, Framework)
	}
}
