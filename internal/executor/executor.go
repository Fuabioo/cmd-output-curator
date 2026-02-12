package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Fuabioo/coc/internal/filter"
	"github.com/Fuabioo/coc/internal/logpath"
)

// smallOutputThreshold is the byte count below which a log file is considered
// not worth keeping (roughly ~80 lines of typical terminal output).
const smallOutputThreshold = 4096

// syncWriter serializes concurrent writes to an io.Writer.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// Config holds the execution configuration.
type Config struct {
	Command  string
	Args     []string
	LogDir   string
	NoFilter bool
	NoLog    bool
	Verbose  bool
	Registry *filter.Registry
}

// Result holds the execution result.
type Result struct {
	ExitCode int
	LogPath  string
}

// Run executes the command with the MultiWriter tee pattern.
func Run(cfg Config) Result {
	command := filepath.Base(cfg.Command)

	// Resolve filter strategy
	strategy := cfg.Registry.Find(command, cfg.Args)
	if cfg.NoFilter || cfg.NoLog {
		strategy = &filter.PassthroughStrategy{}
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "coc: command=%s args=%v filter=%s\n", command, cfg.Args, strategy.Name())
	}

	// Set up log file
	var logFile *os.File
	var logFilePath string
	if !cfg.NoLog {
		logFilePath = logpath.Resolve(cfg.LogDir, command, cfg.Args)
		var err error
		logFile, err = logpath.CreateLogFile(logFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "coc: warning: could not create log file: %v\n", err)
		}
		if cfg.Verbose && logFile != nil {
			fmt.Fprintf(os.Stderr, "coc: log=%s\n", logFilePath)
		}
	}
	// NOTE: no defer logFile.Close() — we manage close explicitly to support
	// the small-output cleanup path without double-close.

	// Wrap logFile in a syncWriter so concurrent stdout/stderr goroutines
	// don't interleave writes.
	var logWriter io.Writer
	if logFile != nil {
		logWriter = &syncWriter{w: logFile}
	}

	// Set up command
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Stdin = os.Stdin

	// Set up stdout capture
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "coc: error creating stdout pipe: %v\n", err)
		if logFile != nil {
			logFile.Close()
		}
		return Result{ExitCode: 1}
	}

	// Set up stderr capture
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "coc: error creating stderr pipe: %v\n", err)
		if logFile != nil {
			logFile.Close()
		}
		return Result{ExitCode: 1}
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "coc: error starting command: %v\n", err)
		if logFile != nil {
			logFile.Close()
		}
		if isNotFound(err) {
			return Result{ExitCode: 127}
		}
		return Result{ExitCode: 1}
	}

	// Set up signal forwarding
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	// Read stdout and stderr concurrently to avoid pipe buffer deadlock.
	// If the child fills stderr (>64KB) while we're blocked draining stdout
	// sequentially, both sides stall. Concurrent reads prevent this.
	var stdoutBuf bytes.Buffer
	var stdoutReader io.Reader = stdoutPipe
	if logWriter != nil {
		stdoutReader = io.TeeReader(stdoutPipe, logWriter)
	}

	var stderrWriters []io.Writer
	stderrWriters = append(stderrWriters, os.Stderr)
	if logWriter != nil {
		stderrWriters = append(stderrWriters, logWriter)
	}
	stderrMulti := io.MultiWriter(stderrWriters...)

	var wg sync.WaitGroup
	wg.Add(2)

	var stdoutCopyErr error
	go func() {
		defer wg.Done()
		_, stdoutCopyErr = io.Copy(&stdoutBuf, stdoutReader)
	}()

	var stderrCopyErr error
	go func() {
		defer wg.Done()
		_, stderrCopyErr = io.Copy(stderrMulti, stderrPipe)
	}()

	wg.Wait()

	// Log warnings for copy errors
	if stdoutCopyErr != nil {
		fmt.Fprintf(os.Stderr, "coc: warning: error reading stdout: %v\n", stdoutCopyErr)
	}
	if stderrCopyErr != nil {
		fmt.Fprintf(os.Stderr, "coc: warning: error reading stderr: %v\n", stderrCopyErr)
	}

	// Wait for command to finish
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				exitCode = 128 + int(status.Signal())
			}
		} else {
			exitCode = 1
		}
	}

	// Apply filter
	result := strategy.Filter(stdoutBuf.Bytes(), command, cfg.Args, exitCode)

	// Write filtered stdout
	if _, err := fmt.Fprint(os.Stdout, result.Filtered); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return Result{ExitCode: exitCode, LogPath: logFilePath}
	}

	// Small output cleanup: if the raw output was small and wasn't reduced,
	// the log file is disk clutter for zero benefit — remove it.
	if logFile != nil && !result.WasReduced && stdoutBuf.Len() <= smallOutputThreshold {
		logFile.Close()
		logFile = nil // prevent double close below
		if err := os.Remove(logFilePath); err == nil {
			// Try to remove parent dir if empty (ignore error — may not be empty)
			_ = os.Remove(filepath.Dir(logFilePath))
		}
		logFilePath = "" // suppress footer
	}

	// Normal cleanup — close if not already closed by small-output path
	if logFile != nil {
		logFile.Close()
	}

	// Write footer if output was reduced
	if result.WasReduced && logFilePath != "" {
		fmt.Fprintf(os.Stderr, "\nOutput was reduced, see the full logs at %s\n", logFilePath)
	}

	return Result{ExitCode: exitCode, LogPath: logFilePath}
}

// isNotFound checks if the error is a command-not-found error.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if pathErr, ok := err.(*exec.Error); ok {
		return pathErr.Err == exec.ErrNotFound
	}
	return false
}
