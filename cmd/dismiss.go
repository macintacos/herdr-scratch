package cmd

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

var dismissCmd = &cobra.Command{
	Use:   "dismiss",
	Short: "Close the popup from inside it, leaving the shell running",
	Long: `Bound to a key by the shell integration.

A herdr popup receives all terminal input until its command exits, so no herdr
keybinding reaches one — whatever runs inside is the only thing positioned to
close it. Detaching ends the client, which is that command, so the popup closes
and the session carries on.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("TMUX") == "" {
			return errors.New("not inside a scratch popup")
		}

		session, err := currentSession()
		if err != nil {
			return err
		}

		// $TMUX already names the server, so no -L is needed here.
		detach := exec.Command("tmux", scratch.DetachArgs(session)...)
		detach.Stderr = os.Stderr
		return detach.Run()
	},
}

// currentSession asks tmux which session this shell is running in, so the
// detach can name it. Without a name tmux has no client to resolve and silently
// detaches nothing.
func currentSession() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", err
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return "", errors.New("could not determine the tmux session")
	}
	return session, nil
}

func init() { rootCmd.AddCommand(dismissCmd) }
