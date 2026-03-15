package bootstrap

import (
	"os"
	"path/filepath"
	"regexp"
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
		pattern := "(?m)^" + regexp.QuoteMeta(target) + "$"
		matched, err := regexp.MatchString(pattern, got)
		if err != nil {
			t.Fatalf("invalid regexp %q: %v", pattern, err)
		}
		if !matched {
			t.Fatalf("expected makefile to include %s target", target)
		}
	}
}
