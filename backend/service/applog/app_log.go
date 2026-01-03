package applog

import (
	"errors"
	"io"
	"os"
	"time"
)

const maxAppLogChunkBytes int64 = 512 * 1024

type AppLogSnapshot struct {
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

func LogsSince(path string, since int64, pid int, startedAt time.Time) AppLogSnapshot {
	snap := AppLogSnapshot{
		Running: true,
		Pid:     pid,
		Path:    path,
		Text:    "",
	}
	if !startedAt.IsZero() {
		snap.StartedAt = startedAt.Format(time.RFC3339Nano)
	}
	if path == "" {
		return snap
	}

	from, to, end, lost, text, err := readLogChunk(path, since, maxAppLogChunkBytes)
	snap.From = from
	snap.To = to
	snap.End = end
	snap.Lost = lost
	snap.Text = text
	if err != nil {
		snap.Error = err.Error()
	}
	return snap
}

func readLogChunk(path string, since, maxBytes int64) (from, to, end int64, lost bool, text string, err error) {
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
