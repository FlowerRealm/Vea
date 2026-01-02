package shared

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ProgressCallback 下载进度回调
// downloaded: 已下载字节数, total: 总字节数, percent: 百分比(0-100)
type ProgressCallback func(downloaded, total int64, percent int)

// ReleaseAssetInfo GitHub Release 资源信息
type ReleaseAssetInfo struct {
	Name        string
	DownloadURL string
	Version     string
	Size        int64
}

// GithubUserAgent 返回 GitHub API 的 User-Agent
func GithubUserAgent() string {
	return fmt.Sprintf("Vea/%s (%s; %s)", "1.3.0", runtime.GOOS, runtime.GOARCH)
}

// FetchLatestReleaseTag 获取最新 release 版本号
func FetchLatestReleaseTag(repo string) (tag string, releaseURL string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", GithubUserAgent())
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := HTTPClientDirect.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var result struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	return result.TagName, result.HTMLURL, nil
}

// BuildDownloadURL 构造下载 URL (v2rayN 风格)
// repo: "SagerNet/sing-box"
// version: "v1.12.12" 或 "1.12.12"
// template: "sing-box-*-linux-amd64.tar.gz"
func BuildDownloadURL(repo, version, template string) (downloadURL, assetName string, err error) {
	if repo == "" || version == "" || template == "" {
		return "", "", errors.New("invalid parameters")
	}

	// 确保版本号格式正确
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	versionNumber := strings.TrimPrefix(tag, "v")

	// 替换模板中的 * 为版本号
	assetName = strings.ReplaceAll(template, "*", versionNumber)
	downloadURL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, assetName)

	return downloadURL, assetName, nil
}

// GetComponentDownloadInfo 获取组件下载信息
func GetComponentDownloadInfo(repo string, candidates []string) (ReleaseAssetInfo, error) {
	if repo == "" || len(candidates) == 0 {
		return ReleaseAssetInfo{}, errors.New("invalid repo or candidates")
	}

	// 获取最新版本号
	tag, _, err := FetchLatestReleaseTag(repo)
	if err != nil {
		log.Printf("[Download] 获取版本号失败: %v", err)
		return ReleaseAssetInfo{}, err
	}

	log.Printf("[Download] 获取到最新版本: %s", tag)

	// 使用第一个候选模板构造下载 URL
	template := candidates[0]
	downloadURL, assetName, err := BuildDownloadURL(repo, tag, template)
	if err != nil {
		return ReleaseAssetInfo{}, err
	}

	log.Printf("[Download] 构造下载 URL: %s", downloadURL)

	return ReleaseAssetInfo{
		Name:        assetName,
		DownloadURL: downloadURL,
		Version:     tag,
		Size:        0, // 下载时会知道
	}, nil
}

// DownloadWithProgress 下载资源并报告进度
func DownloadWithProgress(source string, onProgress ProgressCallback) ([]byte, string, error) {
	return downloadWithUA(source, GithubUserAgent(), onProgress)
}

func downloadWithUA(source, userAgent string, onProgress ProgressCallback) ([]byte, string, error) {
	if source == "" {
		return nil, "", errors.New("empty source url")
	}

	log.Printf("[Download] 开始下载: %s", source)

	doRequest := func(client *http.Client) (*http.Response, error) {
		req, reqErr := http.NewRequest(http.MethodGet, source, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", userAgent)
		return client.Do(req)
	}

	// 尝试不同的 HTTP 客户端
	resp, err := doRequest(HTTPClientDirect)
	if err != nil {
		resp, err = doRequest(HTTPClientDirectHTTP11)
	}
	if err != nil {
		resp, err = doRequest(HTTPClient)
		if err != nil {
			resp, err = doRequest(HTTPClientHTTP11)
		}
	}
	if err != nil {
		log.Printf("[Download] 请求失败: %v", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[Download] HTTP 状态码错误: %s", resp.Status)
		return nil, "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	// 获取文件大小
	contentLength := resp.ContentLength
	log.Printf("[Download] 文件大小: %d bytes (%.2f MB)", contentLength, float64(contentLength)/(1024*1024))

	if contentLength > MaxDownloadSize {
		return nil, "", fmt.Errorf("resource exceeds max size of %d bytes", MaxDownloadSize)
	}

	// 使用 buffer 读取并报告进度
	var buf bytes.Buffer
	var downloaded int64
	var lastReportedPercent int = -1

	bufferSize := 64 * 1024
	buffer := make([]byte, bufferSize)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			buf.Write(buffer[:n])
			downloaded += int64(n)

			// 计算百分比并回调
			var percent int
			if contentLength > 0 {
				percent = int(float64(downloaded) / float64(contentLength) * 100)
				if percent > 100 {
					percent = 100
				}
			}

			// 每1%更新一次
			if onProgress != nil && (percent != lastReportedPercent || percent == 100) {
				onProgress(downloaded, contentLength, percent)
				lastReportedPercent = percent
			}

			// 检查大小限制
			if downloaded > MaxDownloadSize {
				return nil, "", fmt.Errorf("resource exceeds max size of %d bytes", MaxDownloadSize)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
	}

	data := buf.Bytes()
	checksum := ChecksumBytes(data)

	log.Printf("[Download] 下载完成: %d bytes, checksum: %s", len(data), checksum[:16])

	return data, checksum, nil
}

// ChecksumBytes 计算字节数组的 SHA256 校验和
func ChecksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// InferArchiveType 根据文件名推断压缩格式
func InferArchiveType(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(trimmed, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(trimmed, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(trimmed, ".zip"):
		return "zip"
	default:
		return "raw"
	}
}

// ExtractArchive 解压归档文件到目标目录
func ExtractArchive(targetDir, archiveType string, data []byte) (string, error) {
	if targetDir == "" {
		return "", errors.New("install dir is empty")
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return "", err
	}

	tmpDir := targetDir + ".tmp"

	if err := os.RemoveAll(tmpDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}

	var installErr error
	switch archiveType {
	case "raw":
		target := filepath.Join(tmpDir, ComponentFile)
		if err := os.WriteFile(target, data, 0o755); err != nil {
			installErr = err
		}
	case "zip":
		reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			installErr = err
			break
		}
		for _, file := range reader.File {
			if err := extractZipEntry(file, tmpDir); err != nil {
				installErr = err
				break
			}
		}
	case "tar.gz", "tgz":
		if err := extractTarGz(data, tmpDir); err != nil {
			installErr = err
		}
	default:
		installErr = fmt.Errorf("unsupported archive type %s", archiveType)
	}

	if installErr != nil {
		_ = os.RemoveAll(tmpDir)
		return "", installErr
	}

	if err := os.RemoveAll(targetDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(tmpDir, targetDir); err != nil {
		return "", err
	}

	return targetDir, nil
}

func extractZipEntry(file *zip.File, baseDir string) error {
	cleanName := filepath.Clean(file.Name)
	if cleanName == "." {
		return nil
	}

	targetPath, err := safeJoin(baseDir, cleanName)
	if err != nil {
		return err
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := file.Mode()
	if mode == 0 {
		mode = 0o755
	}

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}

func extractTarGz(data []byte, baseDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if cleanName == "." {
			continue
		}

		targetPath, err := safeJoin(baseDir, cleanName)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func safeJoin(baseDir, rel string) (string, error) {
	target := filepath.Join(baseDir, rel)
	baseDirClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(target)
	if !strings.HasPrefix(targetClean, baseDirClean+string(os.PathSeparator)) && targetClean != baseDirClean {
		return "", fmt.Errorf("invalid path traversal detected: %s", rel)
	}
	return targetClean, nil
}

// WriteAtomic 原子性写入文件
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()

	// Ensure temp file is gone on any failure path.
	cleanup := func() {
		_ = os.Remove(tmpName)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		// Windows does not allow rename over existing files.
		_ = os.Remove(path)
		if err2 := os.Rename(tmpName, path); err2 != nil {
			cleanup()
			return err2
		}
	}
	return nil
}
