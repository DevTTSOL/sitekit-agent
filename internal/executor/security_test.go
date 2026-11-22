package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOpenRegularFileRejectsFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}

	file, err := openRegularFile(func() (*os.File, error) {
		return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	})
	if err == nil {
		file.Close()
		t.Fatal("FIFO was accepted as a regular file")
	}
	if err.Error() != "path is not a regular file" {
		t.Fatalf("unexpected error for FIFO: %v", err)
	}
}

func TestConfinedRootRejectsSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}

	if isPathWithin(secret, base) {
		t.Fatal("path in a prefix-adjacent directory was accepted")
	}
	if isPathWithin(filepath.Join(base, "escape", "secret.txt"), base) {
		t.Fatal("symlink escaping the root was accepted")
	}

	root, _, err := openRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := root.ReadFile(filepath.Join("escape", "secret.txt")); err == nil {
		t.Fatal("confined root followed a symlink outside its directory")
	}
}

func TestMigrateDeploymentSymlinks(t *testing.T) {
	appPath := t.TempDir()
	releasePath := filepath.Join(appPath, "releases", "legacy")
	sharedPath := filepath.Join(appPath, "shared")
	for _, path := range []string{
		releasePath,
		sharedPath,
		filepath.Join(sharedPath, "storage"),
		filepath.Join(sharedPath, "node_modules"),
	} {
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sharedPath, ".env"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(releasePath, filepath.Join(appPath, "current")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"storage", "node_modules"} {
		if err := os.Symlink(filepath.Join(sharedPath, path), filepath.Join(releasePath, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(sharedPath, ".env"), filepath.Join(releasePath, ".env")); err != nil {
		t.Fatal(err)
	}

	if err := migrateDeploymentSymlinks(appPath, []string{"storage", "node_modules", ".env"}); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		link   string
		target string
	}{
		{filepath.Join(appPath, "current"), releasePath},
		{filepath.Join(releasePath, "storage"), filepath.Join(sharedPath, "storage")},
		{filepath.Join(releasePath, "node_modules"), filepath.Join(sharedPath, "node_modules")},
		{filepath.Join(releasePath, ".env"), filepath.Join(sharedPath, ".env")},
	} {
		target, err := os.Readlink(test.link)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.IsAbs(target) || filepath.Clean(filepath.Join(filepath.Dir(test.link), target)) != test.target {
			t.Errorf("symlink %q has target %q, want relative path to %q", test.link, target, test.target)
		}
	}

	root, _, err := openRoot(appPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if content, err := root.ReadFile(filepath.Join("current", ".env")); err != nil || string(content) != "secret" {
		t.Fatalf("confined root could not access migrated shared file: %q, %v", content, err)
	}
	if info, err := root.Stat(filepath.Join("current", "storage")); err != nil || !info.IsDir() {
		t.Fatalf("confined root could not access migrated shared directory: %v", err)
	}
}

func TestParseUFWCommandRejectsShellSyntax(t *testing.T) {
	valid := []string{
		"ufw allow 22/tcp",
		"ufw delete allow 443/tcp",
		"ufw route deny from 10.0.0.0/8 to any port 22 proto tcp",
	}
	for _, command := range valid {
		if _, err := parseUFWCommand(command); err != nil {
			t.Errorf("valid command %q was rejected: %v", command, err)
		}
	}

	invalid := []string{
		"bash -c id",
		"ufw allow 22/tcp; id",
		"ufw allow $(id)",
		"ufw allow 22/tcp && touch /tmp/pwned",
		"ufw reset",
	}
	for _, command := range invalid {
		if _, err := parseUFWCommand(command); err == nil {
			t.Errorf("unsafe command %q was accepted", command)
		}
	}
}

func TestSQLQuotingRejectsIdentifiersAndEscapesStrings(t *testing.T) {
	unsafeIdentifiers := []string{"", "db;DROP DATABASE app", "../app", "app.name"}
	for _, identifier := range unsafeIdentifiers {
		if _, err := quoteMySQLIdentifier(identifier); err == nil {
			t.Errorf("unsafe MySQL identifier %q was accepted", identifier)
		}
		if _, err := quotePostgresIdentifier(identifier); err == nil {
			t.Errorf("unsafe PostgreSQL identifier %q was accepted", identifier)
		}
	}

	quoted, err := quoteMySQLString(`password'); DROP DATABASE app; --`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(quoted, `password''); DROP`) {
		t.Fatalf("MySQL password was not safely quoted: %s", quoted)
	}
}

func TestCommandOptionsPassInputWithoutShell(t *testing.T) {
	executor := NewExecutor(time.Second, DatabaseConfig{}, DatabaseConfig{})
	output, exitCode, err := executor.runCommandWithExitCode(
		context.Background(),
		"cat",
		nil,
		commandOptions{stdin: strings.NewReader("payload; $(id)\n")},
	)
	if err != nil || exitCode != 0 {
		t.Fatalf("command failed: %v (exit %d)", err, exitCode)
	}
	if output != "payload; $(id)\n" {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestIsSSHRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   bool
	}{
		{"git@github.com:org/repo.git", true},
		{"deploy@git.example:org/repo.git", true},
		{"git.example:org/repo.git", true},
		{"ssh://git.example/org/repo.git", true},
		{"ssh://deploy@git.example:2222/org/repo.git", true},
		{"https://git.example/org/repo.git", false},
		{"git://git.example/org/repo.git", false},
		{"/srv/git/repo.git", false},
		{"../repo.git", false},
		{"./repo:with-colon.git", false},
	}
	for _, test := range tests {
		if got := isSSHRemote(test.remote); got != test.want {
			t.Errorf("isSSHRemote(%q) = %t, want %t", test.remote, got, test.want)
		}
	}
}

func TestMySQLDefaultsQuoteCredentialsAndRejectNewlines(t *testing.T) {
	defaults, err := mysqlDefaultsContent(`sitekit`, `pass"\line`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(defaults, " -p") || !strings.Contains(defaults, `pass\"\\line`) {
		t.Fatalf("unexpected defaults content: %q", defaults)
	}
	if _, err := mysqlDefaultsContent("sitekit", "one\ntwo"); err == nil {
		t.Fatal("newline in a credential was accepted")
	}
}
