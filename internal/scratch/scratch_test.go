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

func TestLogPathPrefersXDGStateHome(t *testing.T) {
	env := map[string]string{"XDG_STATE_HOME": "/xdg"}
	got := LogPath(func(k string) string { return env[k] }, "/home/me")
	if want := "/xdg/herdr-scratch/herdr-scratch.log"; got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}
}

func TestLogPathFallsBackToTheXDGDefault(t *testing.T) {
	// A log is state rather than data, so it belongs under .local/state rather
	// than .local/share.
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
	got := CreateArgs(false, "/root/tmux.conf", "wD", "/bin/zsh", "/root", 2000)
	want := []string{
		"-f", "/root/tmux.conf", "new-session", "-d", "-s", "wD",
		"-e", "HERDR_SCRATCH_POPUP=1",
		"-e", "HERDR_SCRATCH_ROOT=/root",
		"-e", "HERDR_SCRATCH_NOTIFY_AFTER=2000",
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
	for _, arg := range CreateArgs(false, "/root/tmux.conf", "wD", "/bin/zsh", "/root", 10000) {
		if arg == "-A" {
			t.Fatalf("CreateArgs() passed -A, which attaches when the session exists")
		}
	}
}

func TestCreateArgsRunsNothingWhenTheSessionIsAlreadyUp(t *testing.T) {
	// The second press of the chord, and every one after it: the session is
	// there and the only thing left to do is attach to it.
	if got := CreateArgs(true, "/root/tmux.conf", "wD", "/bin/zsh", "/root", 10000); got != nil {
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

func TestLoadConfigFallsBackToDefaultsWhenThereIsNoFile(t *testing.T) {
	// The common case: nobody has written a config.toml, and the caller hands
	// over the empty read of a file that is not there. A keypress still has to
	// come away with a usable answer.
	got, err := LoadConfig(nil)
	if err != nil {
		t.Errorf("LoadConfig(nil) error = %v, want nil", err)
	}
	if got != DefaultConfig() {
		t.Errorf("LoadConfig(nil) = %#v, want %#v", got, DefaultConfig())
	}
}

func TestLoadConfigFallsBackWhenTheFileCannotBeParsed(t *testing.T) {
	// A half-typed config must not take the popup down with it. The error is
	// reported for the log, and the Config returned beside it is still usable.
	got, err := LoadConfig([]byte("dismiss = [[[\n"))
	if err == nil {
		t.Error("LoadConfig() error = nil, want a parse failure")
	}
	if got != DefaultConfig() {
		t.Errorf("LoadConfig() = %#v, want %#v", got, DefaultConfig())
	}
}

func TestLoadConfigKeepsDefaultsForKeysTheFileOmits(t *testing.T) {
	// Setting one thing must not silently unset the rest, so the decode starts
	// from the defaults rather than from a zero Config.
	got, err := LoadConfig([]byte("notify_after = 500\n"))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if got.NotifyAfter != 500 {
		t.Errorf("NotifyAfter = %d, want 500", got.NotifyAfter)
	}
	if got.Dismiss != DefaultConfig().Dismiss {
		t.Errorf("Dismiss = %q, want the default %q", got.Dismiss, DefaultConfig().Dismiss)
	}
}

func TestLoadConfigReadsEveryKey(t *testing.T) {
	got, err := LoadConfig([]byte(
		"dismiss = \"C-a ;\"\nnotify_after = 2000\nwidth = \"80%\"\nheight = \"50%\"\n"))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	want := Config{Dismiss: "C-a ;", NotifyAfter: 2000, Width: "80%", Height: "50%"}
	if got != want {
		t.Errorf("LoadConfig() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigAcceptsASizeInCells(t *testing.T) {
	// herdr's PopupSize is an integer of terminal cells or a percentage
	// string, so TOML's own integer has to land in the same field as "70%".
	got, err := LoadConfig([]byte("width = 80\n"))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if got.Width != "80" {
		t.Errorf("Width = %q, want %q", got.Width, "80")
	}
}

func TestLoadConfigRejectsAChordThatIsNotTwoKeys(t *testing.T) {
	// The rule DismissKeys already enforces, applied at load time so the
	// message names the config file — rather than surfacing later as a popup
	// with no way out of it.
	got, err := LoadConfig([]byte("dismiss = \"C-b\"\n"))
	if err == nil {
		t.Error("LoadConfig() error = nil, want a validation failure")
	}
	if got != DefaultConfig() {
		t.Errorf("LoadConfig() = %#v, want %#v", got, DefaultConfig())
	}
}

func TestLoadConfigRejectsANegativeThreshold(t *testing.T) {
	// A negative threshold notifies on every command, which reads as the
	// plugin being broken rather than as the config being wrong.
	got, err := LoadConfig([]byte("notify_after = -1\n"))
	if err == nil {
		t.Error("LoadConfig() error = nil, want a validation failure")
	}
	if got != DefaultConfig() {
		t.Errorf("LoadConfig() = %#v, want %#v", got, DefaultConfig())
	}
}

func TestLoadConfigRejectsASizeHerdrWouldRefuse(t *testing.T) {
	// herdr's own schema caps a percentage at 100% and an integer at 65535.
	// Catching it here turns a pane open herdr rejects into a readable line in
	// the log, and leaves the shipped default in place meanwhile.
	for _, size := range []string{"120%", "0%", "70 %", "70000", "wide"} {
		got, err := LoadConfig([]byte("width = \"" + size + "\"\n"))
		if err == nil {
			t.Errorf("LoadConfig(width = %q) error = nil, want a validation failure", size)
		}
		if got != DefaultConfig() {
			t.Errorf("LoadConfig(width = %q) = %#v, want %#v", size, got, DefaultConfig())
		}
	}
}

func TestLoadConfigRejectsAKeyItDoesNotRecognise(t *testing.T) {
	// A typo that decodes quietly is the failure this file exists to end: the
	// user edits `dismis`, the popup keeps its old chord, and nothing anywhere
	// says why. Reported instead, and the defaults stand.
	got, err := LoadConfig([]byte("dismis = \"C-a ;\"\n"))
	if err == nil {
		t.Error("LoadConfig(typo) error = nil, want the unknown key reported")
	}
	if got != DefaultConfig() {
		t.Errorf("LoadConfig(typo) = %#v, want %#v", got, DefaultConfig())
	}
}

func TestLoadConfigRejectsASizeThatIsNotASize(t *testing.T) {
	// A TOML integer is a size; a TOML boolean is not. Weak typing would turn
	// `true` into "1" and open a one-cell popup without complaining.
	if _, err := LoadConfig([]byte("width = true\n")); err == nil {
		t.Error("LoadConfig(width = true) error = nil, want a decode failure")
	}
}

func TestLoadConfigTreatsAnEmptyChordAsUnset(t *testing.T) {
	// `dismiss = ""` has to reach DismissChord as "no chord set" so it falls
	// through to the default, rather than validating as a chord or binding an
	// empty one — which tmux would take, leaving a popup with no way out.
	got, err := LoadConfig([]byte("dismiss = \"\"\n"))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if got.Dismiss != "" {
		t.Errorf("Dismiss = %q, want empty", got.Dismiss)
	}
	if chord := DismissChord("", false, got); chord != DefaultConfig().Dismiss {
		t.Errorf("DismissChord() = %q, want the default %q", chord, DefaultConfig().Dismiss)
	}
}

func TestConfigPathPrefersTheDirectoryHerdrInjects(t *testing.T) {
	// herdr sets HERDR_PLUGIN_CONFIG_DIR on every plugin command, which is the
	// point of it: the plugin never has to work out where its config lives.
	env := map[string]string{"HERDR_PLUGIN_CONFIG_DIR": "/herdr/config/user.scratch"}
	got := ConfigPath(func(k string) string { return env[k] }, "/home/me")
	if want := "/herdr/config/user.scratch/config.toml"; got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestConfigPathFallsBackToXDGConfigHome(t *testing.T) {
	// A binary run by hand gets none of the HERDR_ variables, and still has to
	// name the same file the popup will read.
	env := map[string]string{"XDG_CONFIG_HOME": "/xdg"}
	got := ConfigPath(func(k string) string { return env[k] }, "/home/me")
	if want := "/xdg/herdr/plugins/config/user.scratch/config.toml"; got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestConfigPathFallsBackToTheXDGDefault(t *testing.T) {
	got := ConfigPath(func(string) string { return "" }, "/home/me")
	if want := "/home/me/.config/herdr/plugins/config/user.scratch/config.toml"; got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestLogPathPrefersTheDirectoryHerdrInjects(t *testing.T) {
	// herdr hands every plugin a state directory already scoped to it, so the
	// log goes straight in rather than under a second herdr-scratch level.
	env := map[string]string{"HERDR_PLUGIN_STATE_DIR": "/herdr/state/user.scratch"}
	got := LogPath(func(k string) string { return env[k] }, "/home/me")
	if want := "/herdr/state/user.scratch/herdr-scratch.log"; got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}
}

func TestDismissChordPrefersAFlagThatWasGiven(t *testing.T) {
	// A manifest somebody hand-edited before the settings moved still passes
	// --dismiss, and it has to keep working — their popup answers that chord.
	got := DismissChord("C-a ;", true, Config{Dismiss: "C-b '"})
	if got != "C-a ;" {
		t.Errorf("DismissChord() = %q, want the flag %q", got, "C-a ;")
	}
}

func TestDismissChordTakesTheFileWhenNoFlagWasGiven(t *testing.T) {
	// The ordinary path once the shipped manifest stopped passing one. Given
	// as an empty flag rather than an absent one, because that is what cobra
	// hands over for a flag whose default is "".
	got := DismissChord("", false, Config{Dismiss: "C-a ;"})
	if got != "C-a ;" {
		t.Errorf("DismissChord() = %q, want the config's %q", got, "C-a ;")
	}
}

func TestDismissChordFallsBackWhenTheFileSetsNoChord(t *testing.T) {
	// No config file, or one that set everything except this. An empty chord
	// reaching tmux is a popup with no way out, so there is always a default.
	got := DismissChord("", false, Config{})
	if want := DefaultConfig().Dismiss; got != want {
		t.Errorf("DismissChord() = %q, want the default %q", got, want)
	}
}

func TestSizeArgsSendsNothingWhenNoSizeIsSet(t *testing.T) {
	// Sending no size is what leaves herdr applying the manifest's shipped
	// default, so somebody who never set one sees no change at all.
	if got := SizeArgs(Config{}); got != nil {
		t.Errorf("SizeArgs() = %#v, want nil", got)
	}
}

func TestSizeArgsSendsOnlyTheDimensionThatWasSet(t *testing.T) {
	got := SizeArgs(Config{Height: "40%"})
	want := []string{"--height", "40%"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SizeArgs() = %#v, want %#v", got, want)
	}
}
