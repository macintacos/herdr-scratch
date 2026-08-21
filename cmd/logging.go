package cmd

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/macintacos/herdr-scratch/internal/scratch"
)

// debug switches the log from lines a person reads to JSON, and lets debug
// records through. Set by --debug on the root command.
var debug bool

// openLog points the default slog logger at the file every subcommand shares.
//
// It returns a closer even when it fails: nothing here runs anywhere a person
// would see an error — these are keypresses and prompt hooks — so a log that
// cannot be opened is discarded rather than turned into a failed command.
func openLog(command string) io.Closer {
	sink, closer := logSink()

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler = slog.NewTextHandler(sink, opts)
	if debug {
		handler = slog.NewJSONHandler(sink, opts)
	}

	// Every record carries the subcommand and the pid, because a single
	// keypress fans out into several of these processes — toggle spawns herdr,
	// which spawns the pane, which is popup — and reading the file afterwards
	// means telling those apart.
	slog.SetDefault(slog.New(handler).With("cmd", command, "pid", os.Getpid()))
	return closer
}

// logSink is the file to write to, or a discard when there is nowhere to write.
func logSink() (io.Writer, io.Closer) {
	discard := io.NopCloser(nil)

	home, err := os.UserHomeDir()
	if err != nil {
		return io.Discard, discard
	}
	path := scratch.LogPath(os.Getenv, home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return io.Discard, discard
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard, discard
	}
	return file, file
}
