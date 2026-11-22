package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
