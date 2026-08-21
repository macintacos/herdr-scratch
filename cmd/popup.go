package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

var popupCmd = &cobra.Command{
	Use:   "popup",
	Short: "Exec the tmux session the popup hosts",
	Long: `Run by the plugin pane itself; not something to invoke by hand.

tmux, rather than a lighter session manager, is what makes reopening show the
screen the popup had when it closed. abduco and dtach hold a process open but
keep no screen, so they can only hand back a bare prompt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := pluginRoot()
		if err != nil {
			return err
		}

		session := os.Getenv("HERDR_SCRATCH_SESSION")
		if session == "" {
			session = "default"
		}

		tmuxPath, err := exec.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux is required but is not on PATH")
		}

		argv := []string{
			"tmux", "-L", tmuxSocket,
			"-f", filepath.Join(root, "tmux.conf"),
			"new-session", "-A", "-s", session,
		}
		// On re-attach tmux ignores this command, which is correct: the shell
		// it would apply to is already running.
		argv = append(argv, scratch.ShellCommand(os.Getenv("SHELL"), root)...)

		env := append(os.Environ(),
			"HERDR_SCRATCH_POPUP=1",
			"HERDR_SCRATCH_ROOT="+root,
		)
		// Exec rather than spawn: herdr closes the popup when the process it
		// started exits, so that process must be the tmux client itself.
		return syscall.Exec(tmuxPath, argv, env)
	},
}

func init() { rootCmd.AddCommand(popupCmd) }
