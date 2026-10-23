package apptest

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"
)

// CaptureLogs routes the application's log output into a buffer for the rest
// of the test, so a test can assert what is (and is not) logged.
func CaptureLogs(t *testing.T) *LogBuffer {
	t.Helper()
	b := &LogBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return b
}

// LogBuffer is a concurrency-safe log sink.
type LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
