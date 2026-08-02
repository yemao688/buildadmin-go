package conf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureEnvFileCopiesExampleOnlyWhenMissing(t *testing.T) {
	root := t.TempDir()
	example := []byte("APP_PORT=9900\nAPP_TIME_ZONE=Asia/Shanghai\n")
	if err := os.WriteFile(filepath.Join(root, EnvExampleFileName), example, 0600); err != nil {
		t.Fatal(err)
	}

	created, err := EnsureEnvFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("EnsureEnvFile reported no file creation")
	}
	if got, err := os.ReadFile(filepath.Join(root, EnvFileName)); err != nil || string(got) != string(example) {
		t.Fatalf("copied env = %q, err=%v", got, err)
	}

	custom := []byte("APP_PORT=9911\n")
	if err := os.WriteFile(filepath.Join(root, EnvFileName), custom, 0600); err != nil {
		t.Fatal(err)
	}
	created, err = EnsureEnvFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("EnsureEnvFile overwrote an existing env file")
	}
	if got, err := os.ReadFile(filepath.Join(root, EnvFileName)); err != nil || string(got) != string(custom) {
		t.Fatalf("existing env = %q, err=%v", got, err)
	}
}

func TestLoadEnvFileDoesNotOverrideExistingEnvironment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, EnvFileName)
	if err := os.WriteFile(path, []byte("BA_TEST_EXISTING=file\nBA_TEST_FROM_FILE=value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BA_TEST_EXISTING", "process")
	t.Cleanup(func() { _ = os.Unsetenv("BA_TEST_FROM_FILE") })

	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("BA_TEST_EXISTING"); got != "process" {
		t.Fatalf("existing environment = %q, want process", got)
	}
	if got := os.Getenv("BA_TEST_FROM_FILE"); got != "value" {
		t.Fatalf("loaded environment = %q, want value", got)
	}
}

func TestResolveAppRuntimeEnvironmentUsesEnvAndFallbacks(t *testing.T) {
	settings := ResolveAppRuntimeEnvironment(func(key string) (string, bool) {
		values := map[string]string{
			"APP_PORT":      " 9911 ",
			"APP_TIME_ZONE": "UTC",
		}
		value, ok := values[key]
		return value, ok
	})
	if settings.Port != "9911" || settings.TimeZone != "UTC" {
		t.Fatalf("settings = %#v, want env values", settings)
	}

	settings = ResolveAppRuntimeEnvironment(func(string) (string, bool) { return "", false })
	if settings.Port != "9900" || settings.TimeZone != "Asia/Shanghai" {
		t.Fatalf("fallback settings = %#v", settings)
	}

	settings = ResolveAppRuntimeEnvironment(func(key string) (string, bool) {
		if key == "APP_PORT" {
			return " ", true
		}
		return "", false
	})
	if settings.Port != "9900" || settings.TimeZone != "Asia/Shanghai" {
		t.Fatalf("empty env fallback settings = %#v", settings)
	}
}
