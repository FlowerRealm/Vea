package theme

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"vea/backend/service/shared"
)

const (
	DefaultMaxZipBytes      int64 = 50 << 20  // 50 MiB
	DefaultMaxUnpackedBytes int64 = 200 << 20 // 200 MiB
	DefaultMaxFiles               = 4000
	DefaultMaxDepth               = 24

	DefaultMaxManifestBytes int64 = 1 << 20 // 1 MiB
)

var (
	ErrInvalidThemeID    = errors.New("invalid theme id")
	ErrThemeNotFound     = errors.New("theme not found")
	ErrThemeMissingIndex = errors.New("theme is missing index.html")

	ErrThemeZipTooLarge = errors.New("theme zip is too large")
	ErrThemeZipInvalid  = errors.New("invalid theme zip")

	ErrThemeManifestInvalid = errors.New("invalid theme manifest")
)

type Options struct {
	UserDataRoot     string
	MaxZipBytes      int64
	MaxUnpackedBytes int64
	MaxFiles         int
	MaxDepth         int
}

type Service struct {
	userDataRoot string

	maxZipBytes      int64
	maxUnpackedBytes int64
	maxFiles         int
	maxDepth         int
}

type ThemeInfo struct {
	ID        string    `json:"id"`
	HasIndex  bool      `json:"hasIndex"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`

	// Entry 是入口文件相对 <userData>/themes 的路径（用 '/' 分隔）。
	// 单主题: "<themeId>/index.html"
	// 主题包: "<packId>/<sub...>/index.html"（由 manifest.json 控制）
	Entry string `json:"entry,omitempty"`

	// PackID/PackName 用于标识该主题来自某个主题包（manifest.json）。
	PackID   string `json:"packId,omitempty"`
	PackName string `json:"packName,omitempty"`

	// Name 是可选的展示名（例如主题包内子主题的 name）。
	Name string `json:"name,omitempty"`
}

var themeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

type themePackManifest struct {
	SchemaVersion int                  `json:"schemaVersion"`
	ID            string               `json:"id,omitempty"`
	Name          string               `json:"name,omitempty"`
	Description   string               `json:"description,omitempty"`
	Version       string               `json:"version,omitempty"`
	Author        string               `json:"author,omitempty"`
	Homepage      string               `json:"homepage,omitempty"`
	License       string               `json:"license,omitempty"`
	Themes        []themePackThemeItem `json:"themes"`
	DefaultTheme  string               `json:"defaultTheme,omitempty"`
}

type themePackThemeItem struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Entry       string `json:"entry"`
	Preview     string `json:"preview,omitempty"`
}

func NewService(opts Options) *Service {
	maxZipBytes := opts.MaxZipBytes
	if maxZipBytes <= 0 {
		maxZipBytes = DefaultMaxZipBytes
	}
	maxUnpackedBytes := opts.MaxUnpackedBytes
	if maxUnpackedBytes <= 0 {
		maxUnpackedBytes = DefaultMaxUnpackedBytes
	}
	maxFiles := opts.MaxFiles
	if maxFiles <= 0 {
		maxFiles = DefaultMaxFiles
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}

	return &Service{
		userDataRoot:     strings.TrimSpace(opts.UserDataRoot),
		maxZipBytes:      maxZipBytes,
		maxUnpackedBytes: maxUnpackedBytes,
		maxFiles:         maxFiles,
		maxDepth:         maxDepth,
	}
}

func (s *Service) ThemesRoot() string {
	if strings.TrimSpace(s.userDataRoot) == "" {
		return ""
	}
	return filepath.Join(s.userDataRoot, "themes")
}

// List 扫描 <userData>/themes 目录，返回已安装主题列表。
// 仅返回 ID 合法的目录项（避免 UI/路径处理遇到不可预期的名字）。
func (s *Service) List(ctx context.Context) ([]ThemeInfo, error) {
	root := s.ThemesRoot()
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("user data dir is not configured")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []ThemeInfo{}, nil
		}
		return nil, err
	}

	themes := make([]ThemeInfo, 0, len(entries))
	for _, ent := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !ent.IsDir() {
			continue
		}

		id := ent.Name()
		if err := validateThemeID(id); err != nil {
			continue
		}

		dir := filepath.Join(root, id)
		manifestPath := filepath.Join(dir, "manifest.json")
		if info, err := os.Stat(manifestPath); err == nil && !info.IsDir() {
			packThemes, err := s.listThemePack(ctx, id, dir, ent)
			if err != nil {
				continue
			}
			themes = append(themes, packThemes...)
			continue
		}

		indexPath := filepath.Join(dir, "index.html")
		_, indexErr := os.Stat(indexPath)

		var updatedAt time.Time
		if info, err := ent.Info(); err == nil {
			updatedAt = info.ModTime()
		}

		themes = append(themes, ThemeInfo{
			ID:        id,
			HasIndex:  indexErr == nil,
			UpdatedAt: updatedAt,
			Entry:     path.Join(id, "index.html"),
		})
	}

	sort.Slice(themes, func(i, j int) bool { return themes[i].ID < themes[j].ID })
	return themes, nil
}

// Delete 删除已安装主题目录（<userData>/themes/<id>）。
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := validateThemeID(id); err != nil {
		return err
	}
	root := s.ThemesRoot()
	if strings.TrimSpace(root) == "" {
		return errors.New("user data dir is not configured")
	}

	target, err := shared.SafeJoin(root, id)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrThemeNotFound
		}
		return err
	}
	return os.RemoveAll(target)
}

// ExportZip 将主题目录打包为 zip 并写入 w，zip 内顶层目录为 <id>/。
func (s *Service) ExportZip(ctx context.Context, id string, w io.Writer) error {
	if err := validateThemeID(id); err != nil {
		return err
	}
	root := s.ThemesRoot()
	if strings.TrimSpace(root) == "" {
		return errors.New("user data dir is not configured")
	}

	themeDir, err := shared.SafeJoin(root, id)
	if err != nil {
		return err
	}
	if err := ensureThemeExportable(themeDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrThemeNotFound
		}
		return err
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	walkFn := func(pathname string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 避免导出符号链接，防止打包到目录外文件。
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to export symlink: %s", pathname)
		}

		rel, err := filepath.Rel(themeDir, pathname)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		rel = filepath.ToSlash(rel)
		zipName := path.Join(id, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			if !strings.HasSuffix(zipName, "/") {
				zipName += "/"
			}
			h := &zip.FileHeader{
				Name:     zipName,
				Method:   zip.Store,
				Modified: info.ModTime(),
			}
			h.SetMode(0o755 | fs.ModeDir)
			_, err := zw.CreateHeader(h)
			return err
		}

		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = zipName
		h.Method = zip.Deflate

		writer, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}

		file, err := os.Open(pathname)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	}

	return filepath.WalkDir(themeDir, walkFn)
}

// ImportZip 导入主题 zip，并安全解压安装到 <userData>/themes/<themeId>/。
//
// 规则：
// - zip 必须包含且仅包含一个顶层目录（该目录名即 themeId）
// - 顶层目录下必须存在 index.html
// - 拒绝路径穿越、符号链接、过深目录、文件数/解压总大小超限
func (s *Service) ImportZip(ctx context.Context, zipPath string) (string, error) {
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return "", errors.New("zip path is required")
	}

	stat, err := os.Stat(zipPath)
	if err != nil {
		return "", err
	}
	if stat.Size() > s.maxZipBytes {
		return "", ErrThemeZipTooLarge
	}

	root := s.ThemesRoot()
	if strings.TrimSpace(root) == "" {
		return "", errors.New("user data dir is not configured")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	if len(reader.File) > s.maxFiles {
		return "", fmt.Errorf("%w: too many files", ErrThemeZipInvalid)
	}

	themeID, err := detectThemeID(reader.File)
	if err != nil {
		return "", err
	}
	if err := validateThemeID(themeID); err != nil {
		return "", err
	}

	var unpackedTotal int64
	for _, file := range reader.File {
		if shouldIgnoreZipEntry(file.Name) {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > uint64(^uint64(0)>>1) {
			return "", fmt.Errorf("%w: file too large", ErrThemeZipInvalid)
		}
		unpackedTotal += int64(file.UncompressedSize64)
		if unpackedTotal > s.maxUnpackedBytes {
			return "", fmt.Errorf("%w: unpacked size exceeds limit", ErrThemeZipInvalid)
		}
	}

	tmpDir, err := os.MkdirTemp(root, ".import-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractZipInto(ctx, reader.File, tmpDir, s.maxUnpackedBytes, s.maxFiles, s.maxDepth); err != nil {
		return "", err
	}

	staged := filepath.Join(tmpDir, themeID)
	virtualID, err := validateStagedThemeDir(staged, themeID)
	if err != nil {
		return "", err
	}

	target, err := shared.SafeJoin(root, themeID)
	if err != nil {
		return "", err
	}

	_ = os.RemoveAll(target)
	if err := os.Rename(staged, target); err != nil {
		return "", err
	}
	if strings.TrimSpace(virtualID) != "" {
		return virtualID, nil
	}
	return themeID, nil
}

func validateThemeID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || !themeIDPattern.MatchString(id) {
		return ErrInvalidThemeID
	}
	return nil
}

func ensureThemeExportable(themeDir string) error {
	if _, err := os.Stat(themeDir); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(themeDir, "index.html")); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(filepath.Join(themeDir, "manifest.json")); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fs.ErrNotExist
}

func detectThemeID(files []*zip.File) (string, error) {
	seen := make(map[string]struct{})

	for _, file := range files {
		if shouldIgnoreZipEntry(file.Name) {
			continue
		}

		clean, err := cleanZipEntryPath(file.Name)
		if err != nil {
			return "", err
		}
		if clean == "" {
			continue
		}

		segments := strings.Split(clean, "/")
		if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
			continue
		}

		if len(segments) == 1 && !file.FileInfo().IsDir() {
			// 只接受“单顶层目录”的 zip，避免平铺文件导致 themeId 不可判定。
			return "", fmt.Errorf("%w: require a single top-level directory", ErrThemeZipInvalid)
		}

		seen[segments[0]] = struct{}{}
	}

	if len(seen) != 1 {
		return "", fmt.Errorf("%w: expected exactly one top-level directory", ErrThemeZipInvalid)
	}

	for id := range seen {
		return id, nil
	}
	return "", fmt.Errorf("%w: empty zip", ErrThemeZipInvalid)
}

func validateStagedThemeDir(stagedDir string, topID string) (string, error) {
	manifestPath := filepath.Join(stagedDir, "manifest.json")
	if info, err := os.Stat(manifestPath); err == nil && !info.IsDir() {
		manifest, err := readThemePackManifest(manifestPath)
		if err != nil {
			return "", err
		}
		def := strings.TrimSpace(manifest.DefaultTheme)
		if def == "" && len(manifest.Themes) > 0 {
			def = strings.TrimSpace(manifest.Themes[0].ID)
		}
		if def == "" {
			return "", ErrThemeManifestInvalid
		}
		found := false
		for _, item := range manifest.Themes {
			if err := validateThemeID(item.ID); err != nil {
				return "", fmt.Errorf("%w: invalid theme id %q", ErrThemeManifestInvalid, item.ID)
			}
			entry, err := cleanManifestEntryPath(item.Entry)
			if err != nil {
				return "", err
			}
			target, err := shared.SafeJoin(stagedDir, filepath.FromSlash(entry))
			if err != nil {
				return "", fmt.Errorf("%w: invalid entry %q", ErrThemeManifestInvalid, item.Entry)
			}
			info, err := os.Stat(target)
			if err != nil || info.IsDir() {
				return "", fmt.Errorf("%w: entry not found %q", ErrThemeManifestInvalid, item.Entry)
			}
			if item.ID == def {
				found = true
			}
		}
		if !found {
			def = strings.TrimSpace(manifest.Themes[0].ID)
		}
		return topID + "/" + def, nil
	}

	indexPath := filepath.Join(stagedDir, "index.html")
	if info, err := os.Stat(indexPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", ErrThemeMissingIndex
		}
		return "", err
	} else if info.IsDir() {
		return "", ErrThemeMissingIndex
	}
	return "", nil
}

func readThemePackManifest(pathname string) (*themePackManifest, error) {
	f, err := os.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > DefaultMaxManifestBytes {
		return nil, ErrThemeManifestInvalid
	}

	raw, err := io.ReadAll(io.LimitReader(f, DefaultMaxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > DefaultMaxManifestBytes {
		return nil, ErrThemeManifestInvalid
	}

	var manifest themePackManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrThemeManifestInvalid, err)
	}

	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("%w: unsupported schemaVersion", ErrThemeManifestInvalid)
	}
	if len(manifest.Themes) == 0 {
		return nil, fmt.Errorf("%w: empty themes", ErrThemeManifestInvalid)
	}

	return &manifest, nil
}

func cleanManifestEntryPath(entry string) (string, error) {
	raw := strings.TrimSpace(entry)
	if raw == "" {
		return "", ErrThemeManifestInvalid
	}
	if strings.Contains(raw, "\\") {
		return "", ErrThemeManifestInvalid
	}
	if strings.HasPrefix(raw, "/") {
		return "", ErrThemeManifestInvalid
	}
	if strings.Contains(raw, ":") {
		return "", ErrThemeManifestInvalid
	}

	clean := path.Clean(raw)
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." || clean == "" {
		return "", ErrThemeManifestInvalid
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrThemeManifestInvalid
	}
	if path.Base(clean) != "index.html" {
		return "", ErrThemeManifestInvalid
	}
	return clean, nil
}

func (s *Service) listThemePack(ctx context.Context, packID string, packDir string, ent fs.DirEntry) ([]ThemeInfo, error) {
	manifestPath := filepath.Join(packDir, "manifest.json")
	manifest, err := readThemePackManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	var updatedAt time.Time
	if info, err := ent.Info(); err == nil {
		updatedAt = info.ModTime()
	}

	items := make([]ThemeInfo, 0, len(manifest.Themes))
	for _, theme := range manifest.Themes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := validateThemeID(theme.ID); err != nil {
			continue
		}

		entry, err := cleanManifestEntryPath(theme.Entry)
		if err != nil {
			continue
		}

		target, err := shared.SafeJoin(packDir, filepath.FromSlash(entry))
		if err != nil {
			continue
		}

		info, err := os.Stat(target)
		if err != nil || info.IsDir() {
			continue
		}

		items = append(items, ThemeInfo{
			ID:        packID + "/" + theme.ID,
			HasIndex:  true,
			UpdatedAt: updatedAt,
			Entry:     path.Join(packID, entry),
			PackID:    packID,
			PackName:  strings.TrimSpace(manifest.Name),
			Name:      strings.TrimSpace(theme.Name),
		})
	}

	return items, nil
}

func cleanZipEntryPath(name string) (string, error) {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, "\\") {
		return "", ErrThemeZipInvalid
	}
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "\\") {
		return "", ErrThemeZipInvalid
	}
	if strings.Contains(raw, ":") {
		// Windows 盘符如 C:\...
		return "", ErrThemeZipInvalid
	}

	clean := path.Clean(raw)
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrThemeZipInvalid
	}

	return clean, nil
}

func shouldIgnoreZipEntry(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return true
	}

	clean := path.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	clean = strings.TrimPrefix(clean, "./")
	if clean == "." {
		return true
	}

	parts := strings.Split(clean, "/")
	if len(parts) == 0 {
		return true
	}

	switch parts[0] {
	case "__MACOSX":
		return true
	}

	base := parts[len(parts)-1]
	switch base {
	case ".DS_Store", "Thumbs.db":
		return true
	}
	return false
}

func extractZipInto(ctx context.Context, files []*zip.File, baseDir string, maxUnpackedBytes int64, maxFiles int, maxDepth int) error {
	var totalWritten int64
	writtenFiles := 0

	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if shouldIgnoreZipEntry(file.Name) {
			continue
		}

		cleanName, err := cleanZipEntryPath(file.Name)
		if err != nil {
			return err
		}
		if cleanName == "" {
			continue
		}

		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: refusing symlink %s", ErrThemeZipInvalid, file.Name)
		}

		if maxDepth > 0 && (strings.Count(cleanName, "/")+1) > maxDepth {
			return fmt.Errorf("%w: path too deep", ErrThemeZipInvalid)
		}

		targetPath, err := shared.SafeJoin(baseDir, filepath.FromSlash(cleanName))
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		writtenFiles++
		if maxFiles > 0 && writtenFiles > maxFiles {
			return fmt.Errorf("%w: too many files", ErrThemeZipInvalid)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}

		remaining := maxUnpackedBytes - totalWritten
		if remaining <= 0 {
			out.Close()
			rc.Close()
			return fmt.Errorf("%w: unpacked size exceeds limit", ErrThemeZipInvalid)
		}

		written, copyErr := io.Copy(out, io.LimitReader(rc, remaining+1))
		out.Close()
		rc.Close()

		if copyErr != nil {
			return copyErr
		}
		totalWritten += written
		if totalWritten > maxUnpackedBytes {
			return fmt.Errorf("%w: unpacked size exceeds limit", ErrThemeZipInvalid)
		}
	}

	return nil
}
