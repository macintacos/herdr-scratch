package scratch

import (
	"reflect"
	"testing"
)

func TestSpaceSessionReadsTheWorkspaceEnvironment(t *testing.T) {
	// herdr sets HERDR_WORKSPACE_ID for every plugin command, and on a
	// workspace.closed event it names the space that closed rather than the one
	// that took focus — which is what lets the same reader serve both.
	env := map[string]string{"HERDR_WORKSPACE_ID": "wD"}
	if got := SpaceSession(func(k string) string { return env[k] }); got != "wD" {
		t.Errorf("SpaceSession() = %q, want %q", got, "wD")
	}
}

func TestSpaceSessionFallsBackToTheContextJSON(t *testing.T) {
	env := map[string]string{
		"HERDR_PLUGIN_CONTEXT_JSON": `{"workspace_id":"wE","focused_pane_id":"wE:p1"}`,
	}
	if got := SpaceSession(func(k string) string { return env[k] }); got != "wE" {
		t.Errorf("SpaceSession() = %q, want %q", got, "wE")
	}
}

func TestSpaceSessionNamesSomethingWhenTheSpaceIsUnknown(t *testing.T) {
	// An empty tmux -s argument is an error, so there has to be a name.
	if got := SpaceSession(func(string) string { return "" }); got != "default" {
		t.Errorf("SpaceSession() = %q, want %q", got, "default")
	}
}

func TestSpaceSessionReplacesTmuxSeparators(t *testing.T) {
	// tmux reads both ':' and '.' as window/pane separators, so a session named
	// with either is unaddressable.
	env := map[string]string{"HERDR_WORKSPACE_ID": "w:D.1"}
	if got := SpaceSession(func(k string) string { return env[k] }); got != "w-D-1" {
		t.Errorf("SpaceSession() = %q, want %q", got, "w-D-1")
	}
}

func TestFocusedCwdReadsShellBindingEnvironment(t *testing.T) {
	// A `type = "shell"` keybinding gets plain environment variables.
	env := map[string]string{"HERDR_ACTIVE_PANE_CWD": "/repo"}
	if got := FocusedCwd(func(k string) string { return env[k] }); got != "/repo" {
		t.Errorf("FocusedCwd() = %q, want %q", got, "/repo")
	}
}

func TestFocusedCwdReadsPluginActionContextJSON(t *testing.T) {
	// A `type = "plugin_action"` keybinding gets a JSON context instead, and
	// none of the HERDR_ACTIVE_* variables. Supporting it is what lets the
	// documented binding name an action id rather than a filesystem path.
	env := map[string]string{
		"HERDR_PLUGIN_CONTEXT_JSON": `{"focused_pane_id":"wD:p2E","focused_pane_cwd":"/some dir"}`,
	}
	if got := FocusedCwd(func(k string) string { return env[k] }); got != "/some dir" {
		t.Errorf("FocusedCwd() = %q, want %q", got, "/some dir")
	}
}

func TestFocusedCwdSurvivesUnparseableContext(t *testing.T) {
	// A keypress must never fail on a context it cannot read; the caller has its
	// own fallback for an empty answer.
	env := map[string]string{"HERDR_PLUGIN_CONTEXT_JSON": "{not json"}
	if got := FocusedCwd(func(k string) string { return env[k] }); got != "" {
		t.Errorf("FocusedCwd() = %q, want empty", got)
	}
}

func TestShellCommandAutoSourcesFishIntegration(t *testing.T) {
	// The popup starts the shell, so it can load the plugin's own bindings —
	// which is what removes the "edit your shell config" setup step entirely.
	got := ShellCommand("/opt/homebrew/bin/fish", "/plugin")
	want := []string{
		"/opt/homebrew/bin/fish",
		"--init-command",
		"source /plugin/shell/herdr-scratch.fish",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShellCommand() = %#v, want %#v", got, want)
	}
}

func TestShellCommandLeavesOtherShellsAlone(t *testing.T) {
	// bash and zsh have no equivalent that does not hijack the user's own rc
	// file, so they are launched untouched and bind things manually.
	got := ShellCommand("/bin/zsh", "/plugin")
	if !reflect.DeepEqual(got, []string{"/bin/zsh"}) {
		t.Errorf("ShellCommand() = %#v, want %#v", got, []string{"/bin/zsh"})
	}
}

func TestShellCommandDefersToTmuxWhenShellIsUnset(t *testing.T) {
	// No argv means tmux uses its own default-shell, which is the right answer
	// when the environment cannot tell us what the user runs.
	if got := ShellCommand("", "/plugin"); got != nil {
		t.Errorf("ShellCommand(\"\") = %#v, want nil", got)
	}
}

func TestNotifyArgsAsksHerdrToPostTheNotification(t *testing.T) {
	// herdr posts through the terminal it is attached to, so the notification
	// arrives as that terminal — its icon, its name, and a click that focuses
	// it. terminal-notifier cannot do that: macOS binds a notification to the
	// bundle that actually posts it, so -sender, -appIcon and -contentImage all
	// leave the sender showing as terminal-notifier.
	got := NotifyArgs("npm run dev", "finished after 8s")
	want := []string{"notification", "show", "npm run dev", "--body", "scratch shell · finished after 8s"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotifyArgs() = %#v, want %#v", got, want)
	}
}

func TestNotifyArgsKeepsATitleWhenTheCommandIsUnknown(t *testing.T) {
	// A notification whose title is empty renders as a blank line, and herdr
	// takes the title as a positional argument, so it cannot simply be dropped.
	got := NotifyArgs("", "finished after 8s")
	if got[2] != "scratch shell" {
		t.Errorf("NotifyArgs() title = %q, want %q", got[2], "scratch shell")
	}
}

func TestDetachArgsTargetsTheSessionNotTheCurrentClient(t *testing.T) {
	// A bare `tmux detach-client` run from inside the pane exits 0 and detaches
	// nothing: there is no "current client" to resolve when the caller is not
	// itself a client. Naming the session detaches the client attached to it,
	// which is the popup.
	got := DetachArgs("wD-p10")
	want := []string{"detach-client", "-s", "wD-p10"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetachArgs() = %#v, want %#v", got, want)
	}
}

func TestStableSourceRewritesACellarPathToItsOptSymlink(t *testing.T) {
	// Homebrew numbers the Cellar directory by version and deletes the old one
	// on upgrade. Symlinking a plugin root at it would pin the registration to
	// a version that stops existing; opt/<formula> is the path Homebrew keeps
	// re-pointing at whatever is current.
	got := StableSource("/opt/homebrew/Cellar/herdr-scratch/0.2.0")
	want := "/opt/homebrew/opt/herdr-scratch"
	if got != want {
		t.Errorf("StableSource() = %q, want %q", got, want)
	}
}

func TestStableSourceLeavesANonCellarPathAlone(t *testing.T) {
	// A git checkout is already a stable directory. Rewriting it would point
	// the link at somewhere that does not exist.
	dir := "/Users/me/.config/herdr/scratch"
	if got := StableSource(dir); got != dir {
		t.Errorf("StableSource(%q) = %q, want it unchanged", dir, got)
	}
}

func TestStableRootPrefersXDGDataHome(t *testing.T) {
	env := map[string]string{"XDG_DATA_HOME": "/xdg"}
	got := StableRoot(func(k string) string { return env[k] }, "/home/me")
	if want := "/xdg/herdr-scratch"; got != want {
		t.Errorf("StableRoot() = %q, want %q", got, want)
	}
}

func TestStableRootFallsBackToTheXDGDefault(t *testing.T) {
	// XDG_DATA_HOME is unset far more often than not, and the directory has to
	// land somewhere predictable either way — herdr records it permanently.
	got := StableRoot(func(string) string { return "" }, "/home/me")
	if want := "/home/me/.local/share/herdr-scratch"; got != want {
		t.Errorf("StableRoot() = %q, want %q", got, want)
	}
}

func TestLogPathPrefersXDGStateHome(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": "/xdg"}
	got := LogPath(func(k string) string { return env[k] }, "/home/me")
	if want := "/xdg/herdr-scratch/herdr-scratch.log"; got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}
}

func TestLogPathFallsBackToTheXDGDefault(t *testing.T) {
	// A log is state rather than data, so it belongs under .local/state — not
	// beside the plugin directory StableRoot builds under .local/share.
	got := LogPath(func(string) string { return "" }, "/home/me")
	if want := "/home/me/.local/state/herdr-scratch/herdr-scratch.log"; got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}
}

func TestSessionIsAttachedTreatsAMissingSessionAsNotAttached(t *testing.T) {
	// tmux exits 0 and prints nothing for a session it cannot resolve, so an
	// empty answer means "no such session" — not "attached". Reading it the
	// other way makes the first press in any new pane try to detach a client
	// that was never there, which fails, so the popup never opens.
	if SessionIsAttached("") {
		t.Error(`SessionIsAttached("") = true, want false`)
	}
}

func TestSessionIsAttachedReadsTheClientCount(t *testing.T) {
	if SessionIsAttached("0\n") {
		t.Error(`SessionIsAttached("0") = true, want false`)
	}
	if !SessionIsAttached("1\n") {
		t.Error(`SessionIsAttached("1") = false, want true`)
	}
}

func TestDismissKeysSplitsTheChord(t *testing.T) {
	lead, key, ok := DismissKeys("C-a ;")
	if !ok || lead != "C-a" || key != ";" {
		t.Errorf("DismissKeys() = (%q, %q, %v), want (%q, %q, true)", lead, key, ok, "C-a", ";")
	}
}

func TestDismissKeysRejectsAnythingButTwoKeys(t *testing.T) {
	// A chord this cannot read would otherwise reach tmux as a malformed
	// bind-key, leaving a popup with no way out of it.
	for _, chord := range []string{"", "C-b", "C-b ' x"} {
		if _, _, ok := DismissKeys(chord); ok {
			t.Errorf("DismissKeys(%q) reported ok, want rejected", chord)
		}
	}
}

func TestTmuxKeyArgEscapesTheCommandSeparator(t *testing.T) {
	// tmux reads a lone ";" as the separator between two commands, so a chord
	// ending in one binds nothing — and `bind-key -T scratch ; detach-client`
	// is two commands tmux is happy to run, which is why it fails quietly.
	if got := TmuxKeyArg(";"); got != `\;` {
		t.Errorf("TmuxKeyArg(%q) = %q, want %q", ";", got, `\;`)
	}
}

func TestTmuxKeyArgLeavesOrdinaryKeysAlone(t *testing.T) {
	for _, key := range []string{"C-b", "'", "F1", "Escape"} {
		if got := TmuxKeyArg(key); got != key {
			t.Errorf("TmuxKeyArg(%q) = %q, want it unchanged", key, got)
		}
	}
}

func TestCreateArgsBuildsADetachedSessionWhenThereIsNone(t *testing.T) {
	// The environment goes on the session with -e because the shell is spawned
	// by the tmux server, not by the client that attaches afterwards. Exporting
	// these around the attach reaches the client and nothing else, so the shell
	// integration, which does nothing unless it sees HERDR_SCRATCH_POPUP, would
	// never load.
	got := CreateArgs(false, "/root/tmux.conf", "wD", "/bin/zsh", "/root")
	want := []string{
		"-f", "/root/tmux.conf", "new-session", "-d", "-s", "wD",
		"-e", "HERDR_SCRATCH_POPUP=1",
		"-e", "HERDR_SCRATCH_ROOT=/root",
		"/bin/zsh",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CreateArgs() = %q, want %q", got, want)
	}
}

func TestCreateArgsNeverAsksTmuxToAttach(t *testing.T) {
	// -A turns new-session into attach-session when the session exists, and
	// attaching needs a terminal on stdout. This runs with its output captured,
	// so tmux fails with "open terminal failed: not a terminal" and the popup
	// dies before it ever gets to attach for real.
	for _, arg := range CreateArgs(false, "/root/tmux.conf", "wD", "/bin/zsh", "/root") {
		if arg == "-A" {
			t.Fatalf("CreateArgs() passed -A, which attaches when the session exists")
		}
	}
}

func TestCreateArgsRunsNothingWhenTheSessionIsAlreadyUp(t *testing.T) {
	// The second press of the chord, and every one after it: the session is
	// there and the only thing left to do is attach to it.
	if got := CreateArgs(true, "/root/tmux.conf", "wD", "/bin/zsh", "/root"); got != nil {
		t.Errorf("CreateArgs() = %q, want nil", got)
	}
}

func TestTargetAsksTmuxForAnExactSessionName(t *testing.T) {
	// A bare -t is a prefix match, so with spaces w1 and w12 open, targeting w1
	// can land on w12's session — attaching to the wrong space's shell, or
	// reaping it. tmux's '=' asks for the session actually named this.
	if got := Target("w1"); got != "=w1" {
		t.Errorf("Target() = %q, want %q", got, "=w1")
	}
}

func TestPaneTargetAsksTmuxForAPaneNotASession(t *testing.T) {
	// display-message takes a pane, and a bare session name is not one: given
	// "=wD" it prints nothing and still exits 0, which reads as "not attached"
	// for a session that is. The trailing colon is what makes it a pane target.
	if got := PaneTarget("wD"); got != "=wD:" {
		t.Errorf("PaneTarget() = %q, want %q", got, "=wD:")
	}
}
