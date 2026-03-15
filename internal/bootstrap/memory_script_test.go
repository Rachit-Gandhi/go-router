package bootstrap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCommitJournalEntriesAreStructured(t *testing.T) {
	memoryPath := filepath.Clean("../../memory.md")
	content, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("expected memory file at %s: %v", memoryPath, err)
	}

	start := "<!-- COMMIT_JOURNAL_START -->"
	end := "<!-- COMMIT_JOURNAL_END -->"
	text := string(content)
	startIdx := strings.Index(text, start)
	endIdx := strings.Index(text, end)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatal("expected valid commit journal markers")
	}

	block := text[startIdx+len(start) : endIdx]
	lines := strings.Split(block, "\n")
	entryPattern := regexp.MustCompile(`^- \d{4}-\d{2}-\d{2}T.+ [0-9a-f]{7,}: .+ \[files: .+\]$`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !entryPattern.MatchString(trimmed) {
			t.Fatalf("expected structured commit journal entry, got %q", trimmed)
		}
	}
}

func TestUpdaterScriptUsesPrefixMatchForJournalStart(t *testing.T) {
	scriptPath := filepath.Clean("../../scripts/update_memory_after_commit.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("expected updater script at %s: %v", scriptPath, err)
	}

	if !strings.Contains(string(content), `index($0, start) == 1`) {
		t.Fatal("expected updater script to match start marker using prefix-based check")
	}
}

func TestUpdaterScriptNormalizesEmptyFilesList(t *testing.T) {
	scriptPath := filepath.Clean("../../scripts/update_memory_after_commit.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("expected updater script at %s: %v", scriptPath, err)
	}

	if !strings.Contains(string(content), `if [[ -z "${FILES}" ]]; then`) {
		t.Fatal("expected updater script to handle empty file lists")
	}
	if !strings.Contains(string(content), `FILES="none"`) {
		t.Fatal("expected updater script to normalize empty file lists to a placeholder")
	}
}
