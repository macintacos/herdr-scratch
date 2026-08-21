// Package scratch holds the decisions the herdr-scratch commands are built from,
// separated from the processes they run so they can be tested directly.
package scratch

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// SessionName turns a herdr pane id into a tmux session name.
//
// One session per pane, so each pane keeps its own scratch shell in its own
// directory.
func SessionName(paneID string) string {
	if paneID == "" {
		return "default"
	}
	return strings.NewReplacer(":", "-", ".", "-").Replace(paneID)
}

// PaneTarget reports the herdr pane a keybinding fired from: its id, and the
// directory a new scratch shell should start in.
//
// The two kinds of binding hand this over differently. A `type = "shell"`
// command gets plain environment variables; a `type = "plugin_action"` gets a
// JSON context and none of them. Reading both is what lets the documented
// binding name an action id instead of a path — and a path is the thing that
// breaks when herdr installs the plugin somewhere else.
//
// lookup is the environment reader, injected so this stays testable.
func PaneTarget(lookup func(string) string) (id, cwd string) {
	id, cwd = lookup("HERDR_ACTIVE_PANE_ID"), lookup("HERDR_ACTIVE_PANE_CWD")
	if id != "" && cwd != "" {
		return id, cwd
	}

	var ctx struct {
		PaneID  string `json:"focused_pane_id"`
		PaneCwd string `json:"focused_pane_cwd"`
	}
	// A context we cannot parse leaves the fields empty rather than failing:
	// callers supply their own fallbacks, and a keypress should never error.
	if raw := lookup("HERDR_PLUGIN_CONTEXT_JSON"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &ctx)
	}

	if id == "" {
		id = ctx.PaneID
	}
	if cwd == "" {
		cwd = ctx.PaneCwd
	}
	return id, cwd
}

// ShellCommand is the argv tmux should run for a new scratch session.
//
// Only fish is wired up automatically, through --init-command. That is what
// spares users a shell-config step: the popup starts the shell, so it can load
// the plugin's bindings itself. bash and zsh have no equivalent that does not
// involve displacing the user's own rc file, so they are left untouched.
//
// An empty argv tells tmux to fall back to its configured default-shell.
func ShellCommand(shell, root string) []string {
	if shell == "" {
		return nil
	}
	if filepath.Base(shell) == "fish" {
		return []string{shell, "--init-command",
			"source " + filepath.Join(root, "shell", "herdr-scratch.fish")}
	}
	return []string{shell}
}

// NotifyArgs builds the `herdr notification show` invocation for a finished
// command.
//
// herdr is asked to post it rather than a notifier binary, because macOS binds
// a notification to the bundle that actually posts it. herdr hands the request
// to the terminal it is attached to, so the notification arrives as that
// terminal — its icon, its name, and a click that focuses it. terminal-notifier
// cannot reach that: -sender needs a private API macOS no longer honours,
// -appIcon is ignored, and -contentImage only attaches a thumbnail beside a
// notification still labelled terminal-notifier.
//
// It also means delivery follows whatever the user set in herdr — a desktop
// notification, an in-app toast, or nothing — instead of this plugin deciding.
func NotifyArgs(command, outcome string) []string {
	title := command
	if title == "" {
		title = "scratch shell"
	}
	body := outcome
	if command != "" {
		body = "scratch shell · " + outcome
	}
	return []string{"notification", "show", title, "--body", body}
}

// DetachArgs builds the tmux command that closes the popup.
//
// The session has to be named. A bare `tmux detach-client` run from inside the
// pane exits 0 and detaches nothing, because a caller that is not itself a
// client has no "current client" for tmux to resolve.
func DetachArgs(session string) []string {
	return []string{"detach-client", "-s", session}
}

// StableSource turns the directory a build lives in into one whose path will
// still resolve after the next upgrade.
//
// It exists because herdr resolves a plugin's manifest and records the real
// directory holding it, which under Homebrew is Cellar/<formula>/<version> —
// deleted by the upgrade that replaces it. Homebrew keeps opt/<formula> pointed
// at whatever version is current, so that is what a link should chase.
//
// Anything that is not a Cellar path — a git checkout — is already stable and
// comes back untouched.
func StableSource(root string) string {
	parent, version := filepath.Split(filepath.Clean(root))
	parent, formula := filepath.Split(filepath.Clean(parent))
	if version == "" || formula == "" || filepath.Base(filepath.Clean(parent)) != "Cellar" {
		return root
	}
	return filepath.Join(filepath.Dir(filepath.Clean(parent)), "opt", formula)
}

// StableRoot is the directory herdr is pointed at: one this plugin owns, which
// no upgrade renumbers.
//
// lookup is the environment reader and home the fallback base, both injected so
// this stays testable.
func StableRoot(lookup func(string) string, home string) string {
	if data := lookup("XDG_DATA_HOME"); data != "" {
		return filepath.Join(data, "herdr-scratch")
	}
	return filepath.Join(home, ".local", "share", "herdr-scratch")
}
