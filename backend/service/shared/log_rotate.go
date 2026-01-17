package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RotateLogFile renames an existing log file (if non-empty) into a timestamped file next to it,
// and prunes rotated files older than retain.
//
// Example:
//
//	/path/app.log -> /path/app-20260116-235959.log
func RotateLogFile(path string, retain time.Duration) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" {
		return nil
	}

	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		ts := time.Now().Format("20060102-150405")
		rotated := filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, ts, ext))
		for i := 0; ; i++ {
			if _, err := os.Stat(rotated); err == nil {
				rotated = filepath.Join(dir, fmt.Sprintf("%s-%s-%d%s", stem, ts, i+1, ext))
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
			break
		}
		if err := os.Rename(path, rotated); err != nil {
			return err
		}
	}

	if retain <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-retain)
	prefix := stem + "-"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || (ext != "" && !strings.HasSuffix(name, ext)) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}

	return nil
}
