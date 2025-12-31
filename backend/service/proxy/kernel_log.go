package proxy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vea/backend/domain"
)

const maxKernelLogChunkBytes int64 = 512 * 1024

type KernelLogSnapshot struct {
	Session   uint64 `json:"session"`
	Engine    string `json:"engine,omitempty"`
	Running   bool   `json:"running"`
	Pid       int    `json:"pid,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	Path      string `json:"path,omitempty"`

	From int64  `json:"from"`
	To   int64  `json:"to"`
	End  int64  `json:"end"`
	Lost bool   `json:"lost"`
	Text string `json:"text"`

	Error string `json:"error,omitempty"`
}

func (s *Service) KernelLogsSince(since int64) KernelLogSnapshot {
	s.mu.Lock()
	logPath := s.kernelLogPath
	session := s.kernelLogSession
	engine := s.kernelLogEngine
	startedAt := s.kernelLogStartedAt
	running := s.mainHandle != nil && s.mainHandle.Cmd != nil && s.mainHandle.Cmd.Process != nil
	pid := 0
	if running {
		pid = s.mainHandle.Cmd.Process.Pid
	}
	singBoxLogOutput := ""
	if s.activeCfg.LogConfig != nil {
		singBoxLogOutput = strings.TrimSpace(s.activeCfg.LogConfig.Output)
	}
	configDir := ""
	if logPath != "" {
		configDir = filepath.Dir(logPath)
	}
	s.mu.Unlock()

	// sing-box 允许将日志直接写到文件（log.output）；这种情况下 stdout/stderr 可能为空，
	// UI 想看的“完整日志”应该优先读这个文件。
	readPath := logPath
	if engine == domain.EngineSingBox && singBoxLogOutput != "" && singBoxLogOutput != "stdout" && singBoxLogOutput != "stderr" {
		readPath = singBoxLogOutput
		if !filepath.IsAbs(readPath) && configDir != "" {
			readPath = filepath.Join(configDir, readPath)
		}
	}

	snap := KernelLogSnapshot{
		Session: session,
		Engine:  string(engine),
		Running: running,
		Pid:     pid,
		Path:    readPath,
		Text:    "",
	}
	if !startedAt.IsZero() {
		snap.StartedAt = startedAt.Format(time.RFC3339Nano)
	}

	if readPath == "" {
		return snap
	}

	from, to, end, lost, text, err := readKernelLogChunk(readPath, since, maxKernelLogChunkBytes)
	snap.From = from
	snap.To = to
	snap.End = end
	snap.Lost = lost
	snap.Text = text
	if err != nil {
		snap.Error = err.Error()
	}
	if snap.Error == "" && running && snap.Path != "" && end == 0 {
		if _, statErr := os.Stat(snap.Path); statErr != nil && errors.Is(statErr, os.ErrNotExist) {
			snap.Error = fmt.Sprintf("log file not found: %s", snap.Path)
		}
	}

	return snap
}

func readKernelLogChunk(path string, since, maxBytes int64) (from, to, end int64, lost bool, text string, err error) {
	if maxBytes <= 0 {
		return 0, 0, 0, false, "", errors.New("maxBytes must be > 0")
	}
	if since < 0 {
		since = 0
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, 0, false, "", nil
		}
		return 0, 0, 0, false, "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return 0, 0, 0, false, "", err
	}
	end = st.Size()

	from = since
	if from > end {
		from = 0
		lost = true
	}

	if _, err := f.Seek(from, io.SeekStart); err != nil {
		return 0, 0, 0, false, "", err
	}

	remaining := end - from
	if remaining <= 0 {
		return from, from, end, lost, "", nil
	}

	toRead := remaining
	if toRead > maxBytes {
		toRead = maxBytes
	}

	data, err := io.ReadAll(io.LimitReader(f, toRead))
	if err != nil {
		return 0, 0, 0, false, "", err
	}
	to = from + int64(len(data))
	return from, to, end, lost, string(data), nil
}

type fanoutWriter struct {
	writers []io.Writer
}

func newFanoutWriter(writers ...io.Writer) io.Writer {
	nonNil := make([]io.Writer, 0, len(writers))
	for _, w := range writers {
		if w != nil {
			nonNil = append(nonNil, w)
		}
	}
	return &fanoutWriter{writers: nonNil}
}

func (w *fanoutWriter) Write(p []byte) (int, error) {
	for _, dst := range w.writers {
		_, _ = dst.Write(p)
	}
	return len(p), nil
}

func kernelLogPathForConfigDir(configDir string) string {
	return filepath.Join(configDir, "kernel.log")
}
