package config

import (
	"os"
	"testing"
)

func TestGetDefaultEnvWithPlaceholder(t *testing.T) {
	os.Setenv("ENV", "test")
	env := GetDefaultEnv("ENV", "dev")
	if env != "test" {
		t.Errorf("GetDefaultEnv failed, expect: test, actual: %s", env)
	}
}
