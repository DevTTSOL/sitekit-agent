package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTailLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		lines    int
		expected string
	}{
		{name: "empty", content: "", lines: 1, expected: "\n"},
		{name: "trailing newline", content: "one\ntwo\nthree\n", lines: 2, expected: "two\nthree\n"},
		{name: "no trailing newline", content: "one\ntwo\nthree", lines: 2, expected: "two\nthree\n"},
		{name: "blank last line", content: "one\n\n", lines: 1, expected: "\n"},
		{name: "fewer lines than requested", content: "one", lines: 100, expected: "one\n"},
		{name: "newline across block boundary", content: "one\n" + strings.Repeat("x", 32*1024), lines: 1, expected: strings.Repeat("x", 32*1024) + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "log")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			if _, err := file.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}

			got, err := tailLines(context.Background(), file, tt.lines)
			if err != nil {
				t.Fatalf("tailLines returned an error: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("tailLines = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTailLinesRespectsCancellation(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tailLines(ctx, file, 1); err != context.Canceled {
		t.Fatalf("tailLines error = %v, want %v", err, context.Canceled)
	}
}

func TestFindReleaseForRollbackUsesNewestMatchingRelease(t *testing.T) {
	releasesDir := t.TempDir()
	releaseHash := "0123456789ab"
	olderRelease := filepath.Join(releasesDir, releaseHash)
	newerRelease := filepath.Join(releasesDir, releaseHash+"-attempt")
	for _, path := range []string{olderRelease, newerRelease} {
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
	}

	olderTime := time.Unix(100, 0)
	newerTime := time.Unix(200, 0)
	if err := os.Chtimes(olderRelease, olderTime, olderTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newerRelease, newerTime, newerTime); err != nil {
		t.Fatal(err)
	}

	got, err := findReleaseForRollback(releasesDir, releaseHash)
	if err != nil {
		t.Fatalf("findReleaseForRollback returned an error: %v", err)
	}
	if got != newerRelease {
		t.Fatalf("findReleaseForRollback = %q, want %q", got, newerRelease)
	}
}
