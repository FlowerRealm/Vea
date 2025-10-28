package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"unicode"
)

const githubAPILatestRelease = "https://api.github.com/repos/%s/releases/latest"

var errReleaseAssetNotFound = errors.New("release asset not found")

func fetchLatestReleaseTag(repo string) (tag, version string, err error) {
	if repo == "" {
		return "", "", errors.New("github repo is empty")
	}
	url := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", githubUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	resp.Body.Close()
	finalURL := resp.Request.URL
	if finalURL == nil {
		return "", "", errors.New("release redirect missing final URL")
	}
	segments := strings.Split(strings.Trim(finalURL.Path, "/"), "/")
	if len(segments) == 0 {
		return "", "", errors.New("release redirect missing tag")
	}
	tag = segments[len(segments)-1]
	if tag == "" {
		return "", "", errors.New("empty tag from release redirect")
	}
	version = strings.TrimPrefix(tag, "v")
	return tag, version, nil
}

func probeReleaseAsset(repo, tag, asset string) (bool, int64, error) {
	if asset == "" {
		return false, 0, errors.New("empty asset name")
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("User-Agent", githubUserAgent())
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, resp.ContentLength, nil
	}
	return false, 0, fmt.Errorf("unexpected status %s", resp.Status)
}

type releaseAssetInfo struct {
	Name        string
	DownloadURL string
	Version     string
	Size        int64
}

func githubUserAgent() string {
	return fmt.Sprintf("vea-installer/%s (+https://github.com/flowerrealm/Vea)", strings.TrimPrefix(runtime.Version(), "go"))
}

func fetchLatestReleaseAsset(repo string, candidates []string) (releaseAssetInfo, error) {
	if repo == "" {
		return releaseAssetInfo{}, errors.New("github repo is empty")
	}
	if len(candidates) == 0 {
		return releaseAssetInfo{}, errors.New("no asset candidates provided")
	}
	url := fmt.Sprintf(githubAPILatestRelease, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return releaseAssetInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", githubUserAgent())

	resp, err := httpClient.Do(req)
	if err != nil {
		return releaseAssetInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return releaseAssetInfo{}, fmt.Errorf("github api status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			UpdatedAt          string `json:"updated_at"`
		} `json:"assets"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)) // 8 MiB safety limit
	if err := decoder.Decode(&payload); err != nil {
		return releaseAssetInfo{}, fmt.Errorf("decode github response: %w", err)
	}
	if len(payload.Assets) == 0 {
		return releaseAssetInfo{}, fmt.Errorf("github release has no assets for repo %s", repo)
	}

	candidateLookup := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			candidateLookup = append(candidateLookup, trimmed)
		}
	}
	if len(candidateLookup) == 0 {
		return releaseAssetInfo{}, errors.New("no valid asset candidates provided")
	}

	for _, candidate := range candidateLookup {
		for _, asset := range payload.Assets {
			if !releaseAssetNameMatches(asset.Name, candidate) {
				continue
			}
			info := releaseAssetInfo{
				Name:        asset.Name,
				DownloadURL: asset.BrowserDownloadURL,
				Version:     payload.TagName,
				Size:        asset.Size,
			}
			// Some releases omit browser_download_url; fall back to repository download URL.
			if info.DownloadURL == "" {
				info.DownloadURL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, payload.TagName, asset.Name)
			}
			return info, nil
		}
	}

	return releaseAssetInfo{}, errReleaseAssetNotFound
}

func latestDownloadURL(repo, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, asset)
}

func releaseAssetNameMatches(assetName, candidate string) bool {
	assetLower := strings.ToLower(strings.TrimSpace(assetName))
	candidateLower := strings.ToLower(strings.TrimSpace(candidate))
	if assetLower == candidateLower {
		return true
	}
	if strings.HasSuffix(assetLower, candidateLower) {
		return true
	}
	extensions := []string{".tar.gz", ".tgz", ".zip"}
	for _, ext := range extensions {
		if strings.HasSuffix(assetLower, ext) && strings.HasSuffix(candidateLower, ext) {
			assetBase := strings.TrimSuffix(assetLower, ext)
			candidateBase := strings.TrimSuffix(candidateLower, ext)
			if assetBase == candidateBase {
				return true
			}
			if strings.HasPrefix(assetBase, candidateBase+"-v") {
				return true
			}
		}
	}
	if strings.HasPrefix(candidateLower, "sing-box-") && strings.HasPrefix(assetLower, "sing-box-") {
		suffix := strings.TrimPrefix(candidateLower, "sing-box-")
		if strings.HasSuffix(assetLower, suffix) {
			rest := strings.TrimSuffix(strings.TrimPrefix(assetLower, "sing-box-"), suffix)
			rest = strings.Trim(rest, "-")
			if rest == "" {
				return true
			}
			return looksLikeVersion(rest)
		}
	}
	return false
}

func looksLikeVersion(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsDigit(r) || r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}
