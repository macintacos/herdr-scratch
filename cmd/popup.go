package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

		lead, key, ok := scratch.DismissKeys(dismissChord)
		if !ok {
			return fmt.Errorf("--dismiss wants two tmux keys, like %q, got %q", "C-b '", dismissChord)
		}

		config := filepath.Join(root, "tmux.conf")

		// Create the session detached first, then configure, then attach — three
		// steps rather than one because the chord has to be bound before a
		// client is on the session, and there is no server to bind against until
		// a session exists. `start-server` will not do: an empty server exits
		// the moment it starts.
		exists := sessionExists(session)
		slog.Debug("looked for the space's session", "session", session, "exists", exists)
		if create := scratch.CreateArgs(exists, config, session, os.Getenv("SHELL"), root); create != nil {
			if out, err := tmuxCmd(create...).CombinedOutput(); err != nil {
				slog.Error("could not create the scratch session",
					"session", session, "err", err, "output", strings.TrimSpace(string(out)))
				return fmt.Errorf("could not create the scratch session: %w", err)
			}
			slog.Debug("created the scratch session", "session", session)
		}

		// -f above is read only when tmux has to start a server, so a server
		// that outlives one popup keeps whatever config it started with — an
		// upgraded tmux.conf would never take effect. Sourcing it on every open
		// is what keeps a long-lived server current.
		//
		// Not fatal: a popup that opens with yesterday's config still beats no
		// popup, and the chord below is bound either way.
		if out, err := tmuxCmd("source-file", config).CombinedOutput(); err != nil {
			slog.Error("could not re-read the tmux config",
				"config", config, "err", err, "output", strings.TrimSpace(string(out)))
		}

		// The chord, bound here rather than in tmux.conf because it is the
		// user's choice and that file ships with the plugin. Pressing the lead
		// key twice sends the literal through, so binding it costs that one key
		// and nothing else.
		leadArg, keyArg := scratch.TmuxKeyArg(lead), scratch.TmuxKeyArg(key)
		for _, bind := range [][]string{
			{"bind-key", "-n", leadArg, "switch-client", "-T", dismissTable},
			{"bind-key", "-T", dismissTable, keyArg, "detach-client"},
			{"bind-key", "-T", dismissTable, leadArg, "send-keys", leadArg},
		} {
			if out, err := tmuxCmd(bind...).CombinedOutput(); err != nil {
				slog.Error("could not bind the dismiss chord", "chord", dismissChord,
					"bind", bind, "err", err, "output", strings.TrimSpace(string(out)))
			}
		}
		slog.Debug("bound the dismiss chord", "chord", dismissChord, "lead", lead, "key", key)

		argv := []string{"tmux", "-L", tmuxSocket, "attach-session", "-t", scratch.Target(session)}

		// The last thing this process does as itself. Anything after the exec
		// is tmux, so a log that stops here means tmux took over — and a log
		// that never reaches here means the popup died before it started.
		slog.Info("handing off to tmux",
			"root", root, "session", session, "tmux", tmuxPath,
			"shell", os.Getenv("SHELL"), "argv", argv)
		// Exec rather than spawn: herdr closes the popup when the process it
		// started exits, so that process must be the tmux client itself.
		err = syscall.Exec(tmuxPath, argv, os.Environ())
		slog.Error("exec of tmux returned, which only happens when it failed",
			"tmux", tmuxPath, "err", err)
		return err
	},
}

// sessionExists reports whether the space's scratch session is already up, so
// the popup knows whether there is anything to create.
//
// Only ever asked about existence, never about clients: attaching is this
// command's own last act, and it is the only thing that ever attaches.
func sessionExists(session string) bool {
	return tmuxCmd("has-session", "-t", scratch.Target(session)).Run() == nil
}

// dismissTable is the one-key tmux key table the lead key switches into.
const dismissTable = "scratch"

// dismissChord defaults to herdr's own default prefix and the binding the README
// suggests, so the common setup needs no argument at all.
var dismissChord string

func init() {
	popupCmd.Flags().StringVar(&dismissChord, "dismiss", "C-b '",
		"the two tmux keys that close the popup, matching the chord that opens it")
	rootCmd.AddCommand(popupCmd)
}
