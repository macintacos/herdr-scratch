package cmd

import (
	"fmt"
	"log/slog"
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
			slog.Error("tmux is not on the PATH herdr handed down",
				"path", os.Getenv("PATH"), "err", err)
			return fmt.Errorf("tmux is required but is not on PATH")
		}

		config := filepath.Join(root, "tmux.conf")

		// -f below is read only when tmux has to start a server, so on every
		// attach after the first the running server keeps whatever config it
		// started with — an upgraded tmux.conf would never take effect. Sourcing
		// it here applies it to a server that is already up. It fails when there
		// is none, which is exactly when -f is about to do the job instead.
		if err := tmuxCmd("source-file", config).Run(); err != nil {
			slog.Debug("no running server to re-read the config", "err", err)
		}

		argv := []string{
			"tmux", "-L", tmuxSocket,
			"-f", config,
			"new-session", "-A", "-s", session,
		}
		// On re-attach tmux ignores this command, which is correct: the shell
		// it would apply to is already running.
		argv = append(argv, scratch.ShellCommand(os.Getenv("SHELL"), root)...)

		env := append(os.Environ(),
			"HERDR_SCRATCH_POPUP=1",
			"HERDR_SCRATCH_ROOT="+root,
		)

		// The last thing this process does as itself. Anything after the exec
		// is tmux, so a log that stops here means tmux took over — and a log
		// that never reaches here means the popup died before it started.
		slog.Info("handing off to tmux",
			"root", root, "session", session, "tmux", tmuxPath,
			"shell", os.Getenv("SHELL"), "argv", argv)
		// Exec rather than spawn: herdr closes the popup when the process it
		// started exits, so that process must be the tmux client itself.
		err = syscall.Exec(tmuxPath, argv, env)
		slog.Error("exec of tmux returned, which only happens when it failed",
			"tmux", tmuxPath, "err", err)
		return err
	},
}

func init() { rootCmd.AddCommand(popupCmd) }
