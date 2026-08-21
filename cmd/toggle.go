package cmd

import (
	"os"
	"os/exec"
	"strings"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

const (
	pluginID   = "user.scratch"
	entrypoint = "scratch"
)

var toggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Open the scratch popup, or close the open one",
	Long: `Bound to a key in herdr's config.toml.

Closing is a tmux detach, not a herdr popup.close: detaching ends the client,
which is the process herdr spawned, so herdr tears the popup down by itself. The
session and everything running in it are untouched, and the plugin needs no
socket client of its own.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paneID, paneCwd := scratch.PaneTarget(os.Getenv)
		session := scratch.SessionName(paneID)

		if sessionAttached(session) {
			return tmuxCmd("detach-client", "-s", session).Run()
		}

		if paneCwd == "" {
			paneCwd, _ = os.UserHomeDir()
		}

		open := exec.Command(herdrBin(), "plugin", "pane", "open",
			"--plugin", pluginID,
			"--entrypoint", entrypoint,
			"--cwd", paneCwd,
			"--env", "HERDR_SCRATCH_SESSION="+session,
		)
		open.Stderr = os.Stderr
		return open.Run()
	},
}

// sessionAttached reports whether a scratch session exists and has a client
// attached — which is exactly when its popup is on screen. A missing server or
// session reports false, so the next press opens one.
func sessionAttached(session string) bool {
	out, err := tmuxCmd("display-message", "-p", "-t", session, "#{session_attached}").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != "0"
}

func init() { rootCmd.AddCommand(toggleCmd) }
