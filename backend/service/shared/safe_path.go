package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeJoin 将 baseDir 与 rel 连接，并拒绝路径穿越。
//
// 典型用途是解压归档文件到目标目录，避免出现 "../x"、绝对路径等逃逸 baseDir 的情况。
func SafeJoin(baseDir, rel string) (string, error) {
	target := filepath.Join(baseDir, rel)
	baseDirClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(target)
	if !strings.HasPrefix(targetClean, baseDirClean+string(os.PathSeparator)) && targetClean != baseDirClean {
		return "", fmt.Errorf("invalid path traversal detected: %s", rel)
	}
	return targetClean, nil
}
