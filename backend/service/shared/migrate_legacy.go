package shared

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type LegacyDataMigrationOptions struct {
	// UserDataRoot is the destination root. Empty means UserDataRoot().
	UserDataRoot string

	// LegacyRoots are searched for legacy runtime directories: {root}/data and {root}/artifacts.
	// Empty means: [executableDir(), cwd].
	LegacyRoots []string
}

// MigrateLegacyData moves legacy runtime directories (next to executable / cwd) into UserDataRoot.
//
// Policy:
// - Destination is never overwritten.
// - When the destination already contains the same entry, the legacy entry is deleted.
// - Best-effort: failures are returned but the app can continue to start with the new layout.
func MigrateLegacyData(opts LegacyDataMigrationOptions) error {
	dstRoot := strings.TrimSpace(opts.UserDataRoot)
	if dstRoot == "" {
		dstRoot = UserDataRoot()
	}
	dstRoot = absPath(dstRoot)
	if strings.TrimSpace(dstRoot) == "" {
		return errors.New("user data root is empty")
	}

	dstDataDir := filepath.Join(dstRoot, "data")
	dstArtifactsDir := filepath.Join(dstRoot, "artifacts")

	legacyRoots := opts.LegacyRoots
	if len(legacyRoots) == 0 {
		legacyRoots = defaultLegacyRoots()
	}

	var errs []error
	for _, root := range legacyRoots {
		root = absPath(root)
		if root == "" || root == dstRoot {
			continue
		}

		if err := migrateDir(filepath.Join(root, "data"), dstDataDir); err != nil {
			errs = append(errs, fmt.Errorf("migrate legacy data dir (%s): %w", root, err))
		}
		if err := migrateDir(filepath.Join(root, "artifacts"), dstArtifactsDir); err != nil {
			errs = append(errs, fmt.Errorf("migrate legacy artifacts dir (%s): %w", root, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func defaultLegacyRoots() []string {
	seen := make(map[string]struct{}, 2)
	out := make([]string, 0, 2)
	add := func(p string) {
		p = absPath(p)
		if p == "" {
			return
		}
		if isFilesystemRoot(p) {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(executableDir())
	if cwd, err := os.Getwd(); err == nil {
		// Only treat cwd as legacy root when it looks like the repository root.
		if looksLikeVeaRepoRoot(cwd) {
			add(cwd)
		}
	}
	return out
}

func isFilesystemRoot(path string) bool {
	path = absPath(path)
	if path == "" {
		return false
	}
	path = filepath.Clean(path)
	return filepath.Dir(path) == path
}

func looksLikeVeaRepoRoot(path string) bool {
	path = absPath(path)
	if path == "" {
		return false
	}

	// Heuristic markers of this repository layout.
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "backend")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "frontend")); err != nil {
		return false
	}
	return true
}

func migrateDir(srcDir, dstDir string) error {
	srcDir = absPath(srcDir)
	dstDir = absPath(dstDir)
	if srcDir == "" || dstDir == "" || srcDir == dstDir {
		return nil
	}

	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("not a dir: %s", srcDir)
	}

	// Try fast-path rename when destination does not exist.
	if _, err := os.Stat(dstDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
				return err
			}

			if err := os.Rename(srcDir, dstDir); err == nil {
				log.Printf("[Migrate] moved %s -> %s", srcDir, dstDir)
				return nil
			} else {
				// Rename may fail (e.g. cross-device); fall back to merge to preserve data.
				log.Printf("[Migrate] rename failed (%s -> %s): %v; falling back to merge", srcDir, dstDir, err)
			}
		} else {
			return err
		}
	}

	// Destination exists (or rename failed): merge without overwriting destination.
	if err := mergeDirNoOverwrite(srcDir, dstDir); err != nil {
		return err
	}
	log.Printf("[Migrate] merged %s -> %s (no overwrite)", srcDir, dstDir)
	return nil
}

func mergeDirNoOverwrite(srcDir, dstDir string) error {
	srcDir = absPath(srcDir)
	dstDir = absPath(dstDir)
	if srcDir == "" || dstDir == "" || srcDir == dstDir {
		return nil
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		mode := entry.Type()
		switch {
		case mode&os.ModeSymlink != 0:
			// Prefer keeping destination when it exists; otherwise migrate symlink as-is.
			if _, err := os.Lstat(dstPath); err == nil {
				_ = os.RemoveAll(srcPath)
				continue
			}
			if err := moveSymlink(srcPath, dstPath); err != nil {
				return err
			}

		case entry.IsDir():
			if err := mergeDirNoOverwrite(srcPath, dstPath); err != nil {
				return err
			}

		default:
			if _, err := os.Stat(dstPath); err == nil {
				_ = os.RemoveAll(srcPath)
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			if err := moveFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	// Remove legacy dir after merge. It should be empty now.
	if err := os.RemoveAll(srcDir); err != nil {
		return err
	}
	return nil
}

func moveFile(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		return err
	}
	return os.Remove(srcPath)
}

func moveSymlink(srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	}

	target, err := os.Readlink(srcPath)
	if err != nil {
		return err
	}
	if err := os.Symlink(target, dstPath); err != nil {
		return err
	}
	return os.Remove(srcPath)
}

func copyFile(srcPath, dstPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
