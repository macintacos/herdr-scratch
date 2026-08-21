// Package scratch holds the decisions the herdr-scratch commands are built from,
// separated from the processes they run so they can be tested directly.
package scratch

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// SpaceSession names the tmux session every pane in a space shares.
//
// Keyed on the space rather than the pane so that a scratch shell opened from
// one pane is the same one the next pane in that space gets. It is also what
// bounds the session's life: a space that closes takes its shell with it, which
// a pane-keyed session could not do, since panes come and go under a space that
// stays.
//
// herdr sets HERDR_WORKSPACE_ID on every plugin command, and on a
// workspace.closed event it names the space that closed rather than the one
// that took focus — so this reader serves the toggle and the reap alike. The
// context JSON carries the same id for bindings that get no environment.
//
// lookup is the environment reader, injected so this stays testable.
func SpaceSession(lookup func(string) string) string {
	space := lookup("HERDR_WORKSPACE_ID")
	if space == "" {
		space = readContext(lookup).WorkspaceID
	}
	if space == "" {
		return "default"
	}
	// tmux reads both ':' and '.' as window/pane separators, so a session named
	// with either is unaddressable.
	return strings.NewReplacer(":", "-", ".", "-").Replace(space)
}

// FocusedCwd is the directory a new scratch shell should start in: the one the
// pane that fired the binding was sitting in.
//
// It matters only the first time a space's shell is created. Every open after
// that attaches to a session already running somewhere, which is the point of
// sharing one per space.
//
// The two kinds of binding hand this over differently. A `type = "shell"`
// command gets plain environment variables; a `type = "plugin_action"` gets a
// JSON context and none of them. Reading both is what lets the documented
// binding name an action id instead of a path — and a path is the thing that
// breaks when herdr installs the plugin somewhere else.
func FocusedCwd(lookup func(string) string) string {
	if cwd := lookup("HERDR_ACTIVE_PANE_CWD"); cwd != "" {
		return cwd
	}
	return readContext(lookup).FocusedPaneCwd
}

// herdrContext is the slice of HERDR_PLUGIN_CONTEXT_JSON this plugin reads.
type herdrContext struct {
	WorkspaceID    string `json:"workspace_id"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
}

// readContext parses the context herdr passes, or reports an empty one.
//
// A context that cannot be parsed leaves the fields empty rather than failing:
// callers supply their own fallbacks, and a keypress should never error.
func readContext(lookup func(string) string) herdrContext {
	var ctx herdrContext
	if raw := lookup("HERDR_PLUGIN_CONTEXT_JSON"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &ctx)
	}
	return ctx
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

// LogPath is the file every subcommand logs to.
//
// These run from keypresses and prompt hooks, where stderr goes nowhere anyone
// can read, so a file is the only place a record survives. State rather than
// data, hence .local/state and not the .local/share that StableRoot uses.
//
// HERDR_PLUGIN_STATE_DIR is herdr's answer to the same question, already scoped
// to this plugin, so the log goes straight into it. The XDG tiers below it stay
// for the invocations herdr is not making — `link`, and a binary run by hand.
//
// lookup is the environment reader and home the fallback base, both injected so
// this stays testable.
func LogPath(lookup func(string) string, home string) string {
	if dir := lookup("HERDR_PLUGIN_STATE_DIR"); dir != "" {
		return filepath.Join(dir, "herdr-scratch.log")
	}
	dir := lookup("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "herdr-scratch", "herdr-scratch.log")
}

// SessionIsAttached reads what `tmux display-message -p -t <session>
// "#{session_attached}"` printed, which is true exactly when the session's popup
// is on screen.
//
// The empty case is the one that matters: tmux exits 0 and prints nothing for a
// session it cannot resolve, rather than failing. Treating that as attached
// makes the first press in a pane detach a client that does not exist, so the
// popup never opens.
func SessionIsAttached(out string) bool {
	clients := strings.TrimSpace(out)
	return clients != "" && clients != "0"
}

// DismissKeys splits a dismiss chord into the key that opens tmux's table and
// the key that answers it — "C-b '" being the two keys of ctrl+b then quote.
//
// It is a chord rather than a single key because it has to be the one that
// opened the popup, and herdr's own is a prefix plus a key. Which prefix, and
// which key, are the user's: the default here matches herdr's default, and
// anything else is passed to popup from the manifest.
//
// Reported as not-ok unless it is exactly two keys. tmux would take a malformed
// binding without complaint and leave the popup with no way out.
func DismissKeys(chord string) (lead, key string, ok bool) {
	keys := strings.Fields(chord)
	if len(keys) != 2 {
		return "", "", false
	}
	return keys[0], keys[1], true
}

// TmuxKeyArg spells a key the way tmux's own argument parser needs it.
//
// Only ";" needs the treatment: tmux reads a lone semicolon as the separator
// between two commands, so binding it produces two valid commands and no
// binding — succeeding quietly, which is the worst way for this to fail.
func TmuxKeyArg(key string) string {
	if key == ";" {
		return `\;`
	}
	return key
}

// CreateArgs is the tmux command that puts a space's scratch session in place,
// or nil when it is already there and only wants attaching to.
//
// Deliberately not `new-session -A`: with -A, tmux turns new-session into
// attach-session the moment the session exists, and attaching wants a terminal
// on stdout. This runs with its output captured, so tmux refuses with "open
// terminal failed: not a terminal" and the popup exits before reaching the
// attach it was going to do anyway. Asking first costs one has-session call and
// makes creating and attaching two separate things, which is what they are.
func CreateArgs(exists bool, config, session, shell, root string) []string {
	if exists {
		return nil
	}
	create := []string{"-f", config, "new-session", "-d", "-s", session,
		// On the session, because the shell is spawned by the tmux server and
		// inherits its environment — not the environment of the client that
		// attaches afterwards. Exporting these around the attach reaches the
		// client and nothing else, so the shell integration, which does nothing
		// unless it sees HERDR_SCRATCH_POPUP, would never load.
		//
		// -e rather than the server's own environment: a server outlives the
		// session that started it, so anything inherited from it would be the
		// first space's values for every space after.
		"-e", "HERDR_SCRATCH_POPUP=1",
		"-e", "HERDR_SCRATCH_ROOT=" + root,
	}
	return append(create, ShellCommand(shell, root)...)
}

// Target spells a session name so tmux matches it and nothing else.
//
// tmux resolves a bare -t by exact name, then by prefix, then by pattern. Space
// ids collide under that second rule: with spaces w1 and w12 both open, `-t w1`
// can resolve to w12 — attaching one space to another space's shell, or reaping
// it. The leading '=' asks for exact matching only.
func Target(session string) string {
	return "=" + session
}

// PaneTarget names a session where tmux wants a pane rather than a session.
//
// display-message is the one that cares. Handed a bare session name it prints
// nothing and still exits 0 — indistinguishable from a session with no client,
// so an open popup reads as closed. The trailing colon asks for that session's
// current window and pane, which is a pane target and resolves.
func PaneTarget(session string) string {
	return Target(session) + ":"
}
