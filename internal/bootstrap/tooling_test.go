package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakefileHasDatabaseToolingTargets(t *testing.T) {
	makefilePath := filepath.Clean("../../Makefile")
	content, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("expected makefile at %s: %v", makefilePath, err)
	}

	got := string(content)
	requiredTargets := []string{
		"sqlc-generate:",
		"migrate-up:",
		"migrate-down:",
	}
	for _, target := range requiredTargets {
		if !strings.Contains(got, target) {
			t.Fatalf("expected makefile to include %s target", target)
		}
	}
}
