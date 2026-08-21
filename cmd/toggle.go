package cmd

import (
	"log/slog"
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
		session := scratch.SpaceSession(os.Getenv)
		paneCwd := scratch.FocusedCwd(os.Getenv)
		slog.Debug("resolved the space this fired from",
			"session", session, "pane_cwd", paneCwd,
			"workspace_env", os.Getenv("HERDR_WORKSPACE_ID"),
			"plugin_context_json", os.Getenv("HERDR_PLUGIN_CONTEXT_JSON") != "")

		if sessionAttached(session) {
			slog.Info("closing: session is attached, detaching its client", "session", session)
			err := tmuxCmd("detach-client", "-s", session).Run()
			if err != nil {
				slog.Error("detach failed", "session", session, "err", err)
			}
			return err
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
		slog.Info("opening: no attached session, asking herdr for the pane",
			"session", session, "cwd", paneCwd, "argv", open.Args)

		// Captured rather than passed through: a keybinding's stderr goes to
		// herdr, so the reason herdr refused is otherwise lost.
		out, err := open.CombinedOutput()
		if err != nil {
			slog.Error("herdr would not open the pane",
				"err", err, "output", strings.TrimSpace(string(out)))
			return err
		}
		slog.Debug("herdr opened the pane", "output", strings.TrimSpace(string(out)))
		return nil
	},
}

// sessionAttached reports whether a scratch session exists and has a client
// attached — which is exactly when its popup is on screen. A missing server or
// session reports false, so the next press opens one.
func sessionAttached(session string) bool {
	out, err := tmuxCmd("display-message", "-p", "-t", session, "#{session_attached}").Output()
	if err != nil {
		// Expected the first time a pane is used, and after a reboot. Logged
		// anyway: told apart from "exists but detached", it is the difference
		// between a popup that is new and one that lost its session.
		slog.Debug("no session to attach to", "session", session, "err", err)
		return false
	}
	clients := strings.TrimSpace(string(out))
	attached := scratch.SessionIsAttached(clients)
	slog.Debug("asked tmux about the session",
		"session", session, "attached_clients", clients, "attached", attached,
		"exists", clients != "")
	return attached
}

func init() { rootCmd.AddCommand(toggleCmd) }
