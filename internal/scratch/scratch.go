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

// NotifyArgs builds the terminal-notifier invocation for a finished command.
//
// The icon is set with -appIcon rather than -sender. -sender asks
// terminal-notifier to post as another application, which relies on a private
// bundle-id API macOS no longer honours — the notification simply never
// arrives. -appIcon is the supported way to show the host terminal's face.
//
// An empty icon drops the flag: terminal-notifier rejects the invocation
// outright if -appIcon is given without a value.
func NotifyArgs(subtitle, message, icon string) []string {
	args := []string{
		"-title", "scratch",
		"-subtitle", subtitle,
		"-message", message,
		"-group", "herdr-scratch",
	}
	if icon != "" {
		args = append(args, "-appIcon", icon)
	}
	return args
}

// DetachArgs builds the tmux command that closes the popup.
//
// The session has to be named. A bare `tmux detach-client` run from inside the
// pane exits 0 and detaches nothing, because a caller that is not itself a
// client has no "current client" for tmux to resolve.
func DetachArgs(session string) []string {
	return []string{"detach-client", "-s", session}
}
