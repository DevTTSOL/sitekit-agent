package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var mysqlHostPattern = regexp.MustCompile(`^(?:localhost|%|[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?)$`)

func (e *Executor) mysqlCredentials() (string, string) {
	if e.mysqlConfig.User != "" {
		return e.mysqlConfig.User, e.mysqlConfig.Password
	}

	credentialFiles := []struct {
		user string
		path string
	}{
		{"sitekit", "/opt/sitekit/config/.mysql_sitekit"},
		{"sitekit", "/opt/sitekit/config/.mysql8_sitekit"},
		{"root", "/opt/sitekit/config/.mysql_root"},
		{"root", "/opt/sitekit/config/.mysql8_root"},
	}
	for _, credential := range credentialFiles {
		data, err := os.ReadFile(credential.path)
		if err == nil {
			password := strings.TrimSpace(string(data))
			if password != "" {
				return credential.user, password
			}
		}
	}
	return "", ""
}

func mysqlDefaultsContent(user, password string) (string, error) {
	escape := func(value string) (string, error) {
		if strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n") {
			return "", fmt.Errorf("database credential contains unsupported characters")
		}
		value = strings.ReplaceAll(value, `\`, `\\`)
		return strings.ReplaceAll(value, `"`, `\"`), nil
	}
	escapedUser, err := escape(user)
	if err != nil {
		return "", err
	}
	escapedPassword, err := escape(password)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[client]\nuser=\"%s\"\npassword=\"%s\"\n", escapedUser, escapedPassword), nil
}

func mysqlProgram() string {
	if _, err := os.Stat("/usr/bin/mariadb"); err == nil {
		return "mariadb"
	}
	return "mysql"
}

func (e *Executor) runMySQLProgram(ctx context.Context, program string, args []string, stdin io.Reader, stdout io.Writer) (string, int, error) {
	user, password := e.mysqlCredentials()
	return e.runMySQLProgramWithCredentials(ctx, program, user, password, args, stdin, stdout)
}

func (e *Executor) runMySQLProgramWithCredentials(ctx context.Context, program, user, password string, args []string, stdin io.Reader, stdout io.Writer) (string, int, error) {
	defaultsPath := ""
	if user != "" {
		defaults, err := mysqlDefaultsContent(user, password)
		if err != nil {
			return "", -1, err
		}
		file, err := os.CreateTemp("", "sitekit-mysql-*.cnf")
		if err != nil {
			return "", -1, fmt.Errorf("create MySQL defaults file: %w", err)
		}
		defaultsPath = file.Name()
		defer os.Remove(defaultsPath)
		if _, err := file.WriteString(defaults); err != nil {
			file.Close()
			return "", -1, fmt.Errorf("write MySQL defaults file: %w", err)
		}
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return "", -1, fmt.Errorf("secure MySQL defaults file: %w", err)
		}
		if err := file.Close(); err != nil {
			return "", -1, fmt.Errorf("close MySQL defaults file: %w", err)
		}
		args = append([]string{"--defaults-extra-file=" + defaultsPath}, args...)
	}

	return e.runCommandWithExitCode(ctx, program, args, commandOptions{
		stdin:  stdin,
		stdout: stdout,
	})
}

func (e *Executor) runMySQLQuery(ctx context.Context, query string) (string, int, error) {
	return e.runMySQLProgram(ctx, mysqlProgram(), nil, strings.NewReader(query+"\n"), nil)
}

func (e *Executor) runPostgresProgram(ctx context.Context, program string, args []string, stdin io.Reader, stdout io.Writer) (string, int, error) {
	command := append([]string{"-n", "-u", "postgres", "--", program}, args...)
	return e.runCommandWithExitCode(ctx, "sudo", command, commandOptions{
		stdin:  stdin,
		stdout: stdout,
	})
}

func (e *Executor) runPostgresQuery(ctx context.Context, query string) (string, int, error) {
	return e.runPostgresProgram(ctx, "psql", []string{"--set", "ON_ERROR_STOP=1"}, strings.NewReader(query+"\n"), nil)
}

func validateMySQLHost(host string) error {
	if !mysqlHostPattern.MatchString(host) {
		return fmt.Errorf("invalid database host %q", host)
	}
	return nil
}

func validateBackupFilename(filename string) error {
	if filename == "" || filename != filepathBase(filename) || filename == "." || filename == ".." {
		return fmt.Errorf("invalid backup filename")
	}
	return nil
}

func filepathBase(path string) string {
	index := strings.LastIndexAny(path, `/\`)
	if index >= 0 {
		return path[index+1:]
	}
	return path
}
