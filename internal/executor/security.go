package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	managedUsernamePattern = regexp.MustCompile(`^(?:sitekit|hm_[a-z0-9_-]{1,30})$`)
	sqlIdentifierPattern   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,62}$`)
	ufwTokenPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./:,=-]*$`)
)

func lookupManagedUser(username string) (*user.User, error) {
	if !managedUsernamePattern.MatchString(username) {
		return nil, fmt.Errorf("invalid or unmanaged username %q", username)
	}

	account, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil || uid == 0 {
		return nil, fmt.Errorf("username %q cannot run privileged jobs", username)
	}
	if account.HomeDir == "" || !filepath.IsAbs(account.HomeDir) {
		return nil, fmt.Errorf("username %q has an invalid home directory", username)
	}

	return account, nil
}

func openUserRoot(username string) (*os.Root, string, error) {
	account, err := lookupManagedUser(username)
	if err != nil {
		return nil, "", err
	}
	return openRoot(account.HomeDir)
}

func openRoot(basePath string) (*os.Root, string, error) {
	realBase, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve allowed root: %w", err)
	}
	realBase, err = filepath.Abs(realBase)
	if err != nil {
		return nil, "", fmt.Errorf("resolve allowed root: %w", err)
	}

	info, err := os.Stat(realBase)
	if err != nil {
		return nil, "", fmt.Errorf("inspect allowed root: %w", err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("allowed root is not a directory")
	}

	root, err := os.OpenRoot(realBase)
	if err != nil {
		return nil, "", fmt.Errorf("open allowed root: %w", err)
	}
	return root, realBase, nil
}

func openRegularFile(open func() (*os.File, error)) (*os.File, error) {
	file, err := open()
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("path is not a regular file")
	}
	return file, nil
}

func relativeToRoot(rootPath, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(rootPath, path)
	}
	relative, err := filepath.Rel(rootPath, filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("access denied: path outside allowed directory")
	}
	return relative, nil
}

func relativeSymlinkTarget(linkPath, targetPath string) (string, error) {
	target, err := filepath.Rel(filepath.Dir(linkPath), targetPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlink target: %w", err)
	}
	return target, nil
}

func replaceSymlinkTarget(root *os.Root, linkPath, target string) error {
	for range 10 {
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return err
		}
		tempPath := filepath.Join(filepath.Dir(linkPath), ".sitekit-link-"+hex.EncodeToString(random))
		if err := root.Symlink(target, tempPath); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
		if err := root.Rename(tempPath, linkPath); err != nil {
			root.Remove(tempPath)
			return err
		}
		return nil
	}
	return fmt.Errorf("failed to create a unique temporary symlink")
}

func migrateDeploymentSymlinks(appPath string, sharedPaths []string) error {
	root, realAppPath, err := openRoot(appPath)
	if err != nil {
		return err
	}
	defer root.Close()

	currentInfo, err := root.Lstat("current")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && currentInfo.Mode()&os.ModeSymlink != 0 {
		target, err := root.Readlink("current")
		if err != nil {
			return err
		}
		if filepath.IsAbs(target) {
			target = filepath.Clean(target)
			if _, err := relativeToRoot(filepath.Join(realAppPath, "releases"), target); err == nil {
				relativeTarget, err := relativeSymlinkTarget(filepath.Join(realAppPath, "current"), target)
				if err != nil {
					return err
				}
				if err := replaceSymlinkTarget(root, "current", relativeTarget); err != nil {
					return err
				}
			}
		}
	}

	releases, err := root.Open("releases")
	if err != nil {
		return err
	}
	defer releases.Close()
	entries, err := releases.ReadDir(-1)
	if err != nil {
		return err
	}

	cleanSharedPaths := make([]string, 0, len(sharedPaths))
	for _, path := range sharedPaths {
		if filepath.IsAbs(path) {
			continue
		}
		path = filepath.Clean(path)
		if path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			continue
		}
		cleanSharedPaths = append(cleanSharedPaths, path)
	}
	sort.Slice(cleanSharedPaths, func(i, j int) bool {
		return strings.Count(cleanSharedPaths[i], string(filepath.Separator)) <
			strings.Count(cleanSharedPaths[j], string(filepath.Separator))
	})

	sharedRoot := filepath.Join(realAppPath, "shared")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, sharedPath := range cleanSharedPaths {
			linkPath := filepath.Join("releases", entry.Name(), sharedPath)
			info, err := root.Lstat(linkPath)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			target, err := root.Readlink(linkPath)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(target) {
				continue
			}
			target = filepath.Clean(target)
			if _, err := relativeToRoot(sharedRoot, target); err != nil {
				continue
			}
			absoluteLinkPath := filepath.Join(realAppPath, linkPath)
			relativeTarget, err := relativeSymlinkTarget(absoluteLinkPath, target)
			if err != nil {
				return err
			}
			if err := replaceSymlinkTarget(root, linkPath, relativeTarget); err != nil {
				return err
			}
		}
	}
	return nil
}

func openUserPath(username, path string) (*os.Root, string, string, error) {
	root, realHome, err := openUserRoot(username)
	if err != nil {
		return nil, "", "", err
	}
	relative, err := relativeToRoot(realHome, path)
	if err != nil {
		root.Close()
		return nil, "", "", err
	}
	return root, realHome, relative, nil
}

func validateManagedUserPath(username, path string) (*user.User, string, error) {
	account, err := lookupManagedUser(username)
	if err != nil {
		return nil, "", err
	}

	realHome, err := filepath.EvalSymlinks(account.HomeDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve user home: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve work directory: %w", err)
	}
	relative, err := relativeToRoot(realHome, realPath)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(filepath.Join(realHome, relative))
	if err != nil {
		return nil, "", fmt.Errorf("inspect work directory: %w", err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("work directory is not a directory")
	}
	return account, filepath.Join(realHome, relative), nil
}

func (e *Executor) runCommandAsUser(ctx context.Context, username, directory string, environment []string, name string, args ...string) (string, int, error) {
	_, realDirectory, err := validateManagedUserPath(username, directory)
	if err != nil {
		return "", -1, err
	}

	command := append([]string{"/usr/bin/env"}, environment...)
	command = append(command, name)
	command = append(command, args...)
	sudoArgs := append([]string{"-n", "-H", "-u", username, "--"}, command...)
	return e.runCommandWithExitCode(ctx, "sudo", sudoArgs, commandOptions{dir: realDirectory})
}

func writeUserFile(username, path string, content []byte, mode os.FileMode) error {
	root, _, relative, err := openUserPath(username, path)
	if err != nil {
		return err
	}
	defer root.Close()

	file, err := root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func createUserTempFile(username, directory, prefix string, mode os.FileMode) (*os.File, string, error) {
	root, realHome, relativeDirectory, err := openUserPath(username, directory)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()

	info, err := root.Stat(relativeDirectory)
	if err != nil {
		return nil, "", err
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("temporary directory is not a directory")
	}

	for range 10 {
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return nil, "", err
		}
		relativePath := filepath.Join(relativeDirectory, prefix+hex.EncodeToString(random))
		file, err := root.OpenFile(relativePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			return file, filepath.Join(realHome, relativePath), nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("failed to create unique temporary file")
}

func quoteMySQLIdentifier(identifier string) (string, error) {
	if !sqlIdentifierPattern.MatchString(identifier) || strings.Contains(identifier, ".") {
		return "", fmt.Errorf("invalid database identifier %q", identifier)
	}
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`", nil
}

func quotePostgresIdentifier(identifier string) (string, error) {
	if !sqlIdentifierPattern.MatchString(identifier) || strings.Contains(identifier, ".") {
		return "", fmt.Errorf("invalid database identifier %q", identifier)
	}
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`, nil
}

func quoteMySQLString(value string) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("database value contains a NUL byte")
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `''`)
	return "'" + value + "'", nil
}

func quotePostgresString(value string) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("database value contains a NUL byte")
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'", nil
}

func parseUFWCommand(command string) ([]string, error) {
	parts := strings.Fields(command)
	if len(parts) < 3 || parts[0] != "ufw" {
		return nil, fmt.Errorf("invalid ufw command")
	}

	allowedActions := map[string]bool{
		"allow":   true,
		"deny":    true,
		"reject":  true,
		"limit":   true,
		"delete":  true,
		"route":   true,
		"logging": true,
	}
	if !allowedActions[parts[1]] {
		return nil, fmt.Errorf("unsupported ufw action %q", parts[1])
	}

	for _, token := range parts[1:] {
		if !ufwTokenPattern.MatchString(token) {
			return nil, fmt.Errorf("invalid ufw argument %q", token)
		}
	}
	return parts[1:], nil
}

func parseFileMode(value string, defaultMode os.FileMode) (os.FileMode, error) {
	if value == "" {
		if defaultMode == 0 {
			return 0, fmt.Errorf("file mode is required")
		}
		return defaultMode, nil
	}
	mode, err := strconv.ParseUint(value, 8, 12)
	if err != nil || mode > 0777 {
		return 0, fmt.Errorf("invalid file mode %q", value)
	}
	return os.FileMode(mode), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func sshCommand(keyPath, knownHostsPath string) string {
	parts := []string{
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=yes",
		"-o", "UserKnownHostsFile=" + knownHostsPath,
	}
	if keyPath != "" {
		parts = append(parts, "-o", "IdentitiesOnly=yes", "-i", keyPath)
	}
	for index := range parts {
		parts[index] = shellQuote(parts[index])
	}
	return strings.Join(parts, " ")
}
