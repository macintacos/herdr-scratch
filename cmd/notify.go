package cmd

import (
	"os"
	"os/exec"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

var notifyCmd = &cobra.Command{
	Use:   "notify <command> [outcome]",
	Short: "Post a notification about a command that finished out of sight",
	Long: `Called by the shell integration when a command finishes in a popup
nobody is looking at.

herdr posts it, not this plugin. herdr hands the request to the terminal it is
attached to, so the notification arrives wearing that terminal's icon and name,
and clicking it focuses the terminal. Delivery follows the user's herdr config —
a desktop notification, an in-app toast, or nothing at all.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		outcome := ""
		if len(args) > 1 {
			outcome = args[1]
		}

		herdr := os.Getenv("HERDR_BIN_PATH")
		if herdr == "" {
			herdr = "herdr"
		}

		// Best-effort: this runs from a shell prompt hook, so a notification
		// that cannot be posted must never surface as an error at the prompt.
		post := exec.Command(herdr, scratch.NotifyArgs(args[0], outcome)...)
		_ = post.Run()
		return nil
	},
}

func init() { rootCmd.AddCommand(notifyCmd) }
