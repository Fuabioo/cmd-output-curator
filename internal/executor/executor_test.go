package executor

import (
	"bytes"
	"sync"
	"testing"

	"github.com/Fuabioo/coc/internal/filter"
)

func TestSyncWriter(t *testing.T) {
	var buf bytes.Buffer
	sw := &syncWriter{w: &buf}

	// Write from multiple goroutines to verify thread safety
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sw.Write([]byte("x"))
			if err != nil {
				t.Errorf("Write error: %v", err)
			}
		}()
	}
	wg.Wait()

	if buf.Len() != 100 {
		t.Errorf("Expected 100 bytes, got %d", buf.Len())
	}
}

func TestSyncWriterContent(t *testing.T) {
	var buf bytes.Buffer
	sw := &syncWriter{w: &buf}

	data := []byte("hello world")
	n, err := sw.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("Write returned %d, want %d", n, len(data))
	}
	if buf.String() != "hello world" {
		t.Errorf("Buffer contains %q, want %q", buf.String(), "hello world")
	}
}

func TestSyncWriterEmpty(t *testing.T) {
	var buf bytes.Buffer
	sw := &syncWriter{w: &buf}

	n, err := sw.Write([]byte{})
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write of empty slice returned %d, want 0", n)
	}
}

func TestRunEchoPassthrough(t *testing.T) {
	cfg := Config{
		Command:  "echo",
		Args:     []string{"hello"},
		NoLog:    true,
		Registry: filter.DefaultRegistry(),
	}

	result := Run(cfg)
	if result.ExitCode != 0 {
		t.Errorf("echo should exit 0, got %d", result.ExitCode)
	}
}

func TestRunFalseExitCode(t *testing.T) {
	cfg := Config{
		Command:  "false",
		Args:     nil,
		NoLog:    true,
		Registry: filter.DefaultRegistry(),
	}

	result := Run(cfg)
	if result.ExitCode != 1 {
		t.Errorf("false should exit 1, got %d", result.ExitCode)
	}
}

func TestRunCommandNotFound(t *testing.T) {
	cfg := Config{
		Command:  "nonexistent-command-that-does-not-exist",
		Args:     nil,
		NoLog:    true,
		Registry: filter.DefaultRegistry(),
	}

	result := Run(cfg)
	if result.ExitCode != 127 {
		t.Errorf("nonexistent command should exit 127, got %d", result.ExitCode)
	}
}

func TestRunNoLogMeansNoLogPath(t *testing.T) {
	cfg := Config{
		Command:  "echo",
		Args:     []string{"test"},
		NoLog:    true,
		Registry: filter.DefaultRegistry(),
	}

	result := Run(cfg)
	if result.LogPath != "" {
		t.Errorf("NoLog should result in empty LogPath, got %q", result.LogPath)
	}
}

func TestRunTrueExitCode(t *testing.T) {
	cfg := Config{
		Command:  "true",
		Args:     nil,
		NoLog:    true,
		Registry: filter.DefaultRegistry(),
	}

	result := Run(cfg)
	if result.ExitCode != 0 {
		t.Errorf("true should exit 0, got %d", result.ExitCode)
	}
}

func TestIsNotFound(t *testing.T) {
	// nil error is not "not found"
	if isNotFound(nil) {
		t.Error("isNotFound(nil) should be false")
	}
}
