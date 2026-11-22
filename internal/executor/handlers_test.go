package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
