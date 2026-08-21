package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/macintacos/herdr-scratch/internal/scratch"
	"github.com/spf13/cobra"
)

var notifyCmd = &cobra.Command{
	Use:   "notify <subtitle> [message]",
	Short: "Post a desktop notification about a finished command",
	Long: `Called by the shell integration when a command finishes in a popup
nobody is looking at.

Best-effort throughout: it runs from a prompt hook, so a missing
terminal-notifier or an unresolvable icon must never surface an error.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		notifier, err := exec.LookPath("terminal-notifier")
		if err != nil {
			return nil // not installed: notifications are opt-in
		}

		message := ""
		if len(args) > 1 {
			message = args[1]
		}

		// Never waited on: terminal-notifier stays alive to own the click, and
		// the prompt hook calling this has to return immediately.
		post := exec.Command(notifier, scratch.NotifyArgs(args[0], message, hostAppIcon())...)
		post.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := post.Start(); err != nil {
			return nil
		}
		return post.Process.Release()
	},
}

// hostAppIcon finds the icon of the terminal herdr is running in, so a
// notification looks like it came from there rather than from a notifier binary.
//
// macOS puts the host app's bundle id in __CFBundleIdentifier, and it survives
// down through herdr and tmux into the popup — so the icon follows whichever
// terminal is actually attached instead of being pinned to one. Anything it
// cannot resolve returns "", which leaves the icon alone.
func hostAppIcon() string {
	bundleID := os.Getenv("__CFBundleIdentifier")
	if bundleID == "" {
		return ""
	}

	out, err := exec.Command("osascript", "-e",
		`POSIX path of (path to application id "`+bundleID+`")`).Output()
	if err != nil {
		return ""
	}
	appPath := strings.TrimSpace(string(out))
	if appPath == "" {
		return ""
	}

	icons, err := filepath.Glob(filepath.Join(appPath, "Contents", "Resources", "*.icns"))
	if err != nil || len(icons) == 0 {
		return ""
	}
	return icons[0]
}

func init() { rootCmd.AddCommand(notifyCmd) }
