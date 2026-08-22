// Package cmd wires the herdr-scratch subcommands to the processes they run.
// The decisions they are built from live in internal/scratch, under test.
package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

// tmuxSocket keeps this server clear of any tmux the user runs themselves, and
// lets the plugin apply its own config without touching theirs.
const tmuxSocket = "herdr-scratch"

var rootCmd = &cobra.Command{
	Use:   "herdr-scratch",
	Short: "A scratch shell for herdr, in a popup you toggle with one chord",
	Long: `herdr-scratch backs a herdr popup with a tmux session per space.

Each subcommand is invoked by a different part of herdr: toggle by a keybinding,
popup by the plugin pane itself, dismiss and notify by the shell integration
running inside the popup.`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logCloser = openLog(cmd.Name())
	},
}

// logCloser holds the open log file so Execute can flush it on the way out.
var logCloser io.Closer

// Execute runs the CLI with the version it was built as, reporting a failure as one line on stderr rather than
// cobra's default usage dump — these run from keypresses and prompt hooks.
//
// The failure is logged as well as printed. Most of these processes have no
// stderr anyone will ever read: a keybinding's goes to herdr, and popup's is
// inside a popup that is about to close.
func Execute(version string) {
	rootCmd.Version = version
	// Just the number: this gets read by scripts and compared against
	// `brew info` far more often than it gets read as a sentence.
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	err := rootCmd.Execute()
	if err != nil {
		slog.Error("command failed", "err", err)
		fmt.Fprintln(os.Stderr, "herdr-scratch:", err)
	}
	if logCloser != nil {
		_ = logCloser.Close()
	}
	if err != nil {
		os.Exit(1)
	}
}

// tmuxCmd builds a command against the plugin's own tmux server.
func tmuxCmd(args ...string) *exec.Cmd {
	return exec.Command("tmux", append([]string{"-L", tmuxSocket}, args...)...)
}

// pluginRoot is the directory the plugin is installed in.
//
// herdr sets HERDR_PLUGIN_ROOT and runs plugin commands with it as the working
// directory. The executable's own location is the fallback, for running the
// binary by hand.
func pluginRoot() (string, error) {
	if root := os.Getenv("HERDR_PLUGIN_ROOT"); root != "" {
		return root, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(self)), nil // <root>/bin/herdr-scratch
}

// herdrBin is the herdr to invoke. herdr sets HERDR_BIN_PATH when it runs a
// plugin command, which names the running herdr rather than whichever one PATH
// happens to find.
func herdrBin() string {
	if herdr := os.Getenv("HERDR_BIN_PATH"); herdr != "" {
		return herdr
	}
	return "herdr"
}

// userConfig is the settings the user owns, or the defaults when there is
// nothing to read them from.
//
// Never fails: the file is optional, and every caller here is a keypress. A
// config that cannot be read or does not validate is logged and replaced with
// the defaults, because a popup that opens with the wrong chord still beats a
// popup that does not open.
func userConfig() scratch.Config {
	// A missing home directory is not fatal here: it only feeds ConfigPath's
	// last tier, and HERDR_PLUGIN_CONFIG_DIR — set on every command herdr runs,
	// which is all of them that matter — is resolved before that tier is
	// reached.
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("no home directory; only HERDR_PLUGIN_CONFIG_DIR can name the config", "err", err)
	}

	path := scratch.ConfigPath(os.Getenv, home)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		slog.Error("could not read the config", "path", path, "err", err)
	}

	cfg, err := scratch.LoadConfig(data)
	if err != nil {
		slog.Error("using the built-in settings instead", "path", path, "err", err)
	}
	slog.Debug("read the config", "path", path, "config", cfg)
	return cfg
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
		"write the log as JSON, including debug records")
}
