// Package cmd wires the herdr-scratch subcommands to the processes they run.
// The decisions they are built from live in internal/scratch, under test.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// tmuxSocket keeps this server clear of any tmux the user runs themselves, and
// lets the plugin apply its own config without touching theirs.
const tmuxSocket = "herdr-scratch"

var rootCmd = &cobra.Command{
	Use:   "herdr-scratch",
	Short: "A scratch shell for herdr, in a popup you toggle with one chord",
	Long: `herdr-scratch backs a herdr popup with a per-pane tmux session.

Each subcommand is invoked by a different part of herdr: toggle by a keybinding,
popup by the plugin pane itself, dismiss and notify by the shell integration
running inside the popup.`,
	SilenceUsage: true,
}

// Execute runs the CLI, reporting a failure as one line on stderr rather than
// cobra's default usage dump — these run from keypresses and prompt hooks.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-scratch:", err)
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
