package cmd

import (
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/macintacos/herdr-scratch/internal/scratch"
)

var reapCmd = &cobra.Command{
	Use:   "reap",
	Short: "Kill a closed space's scratch shell",
	Long: `Run by herdr on workspace.closed; not something to invoke by hand.

A scratch shell belongs to its space and outlives every pane in it, so nothing
else would ever end it. Without this the session would sit on the tmux server
holding whatever was running in it, unreachable — the space it was keyed to is
gone, and the next space gets an id of its own.

herdr names the space that closed rather than the one that took focus, which is
what makes the session safe to name from the same reader the toggle uses.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		session := scratch.SpaceSession(os.Getenv)

		// Not an error, and by far the common case: most spaces never open a
		// scratch shell, and every one of them closes.
		out, err := tmuxCmd("kill-session", "-t", scratch.Target(session)).CombinedOutput()
		if err != nil {
			slog.Debug("no scratch shell to reap", "session", session,
				"output", strings.TrimSpace(string(out)))
			return nil
		}
		slog.Info("reaped the closed space's scratch shell", "session", session)
		return nil
	},
}

func init() { rootCmd.AddCommand(reapCmd) }
