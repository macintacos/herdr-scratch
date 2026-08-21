package scratch

import (
	"reflect"
	"testing"
)

func TestSessionNameReplacesTmuxSeparators(t *testing.T) {
	// herdr pane ids contain a colon, and tmux reads both ':' and '.' as
	// window/pane separators — a session named "wD:p10" is unaddressable.
	got := SessionName("wD:p10")
	if got != "wD-p10" {
		t.Errorf("SessionName(%q) = %q, want %q", "wD:p10", got, "wD-p10")
	}
}

func TestSessionNameFallsBackWhenPaneIDIsMissing(t *testing.T) {
	// A pane id is absent when the binding is invoked outside a pane context.
	// An empty tmux -s argument is an error, so it needs a name of its own.
	if got := SessionName(""); got != "default" {
		t.Errorf("SessionName(%q) = %q, want %q", "", got, "default")
	}
}

func TestPaneTargetReadsShellBindingEnvironment(t *testing.T) {
	// A `type = "shell"` keybinding gets plain environment variables.
	env := map[string]string{
		"HERDR_ACTIVE_PANE_ID":  "wD:p10",
		"HERDR_ACTIVE_PANE_CWD": "/repo",
	}
	id, cwd := PaneTarget(func(k string) string { return env[k] })
	if id != "wD:p10" || cwd != "/repo" {
		t.Errorf("PaneTarget() = (%q, %q), want (%q, %q)", id, cwd, "wD:p10", "/repo")
	}
}

func TestPaneTargetReadsPluginActionContextJSON(t *testing.T) {
	// A `type = "plugin_action"` keybinding gets a JSON context instead, and
	// none of the HERDR_ACTIVE_* variables. Supporting it is what lets the
	// documented binding name an action id rather than a filesystem path.
	env := map[string]string{
		"HERDR_PLUGIN_CONTEXT_JSON": `{"workspace_id":"wD","focused_pane_id":"wD:p2E","focused_pane_cwd":"/some dir"}`,
	}
	id, cwd := PaneTarget(func(k string) string { return env[k] })
	if id != "wD:p2E" || cwd != "/some dir" {
		t.Errorf("PaneTarget() = (%q, %q), want (%q, %q)", id, cwd, "wD:p2E", "/some dir")
	}
}

func TestPaneTargetSurvivesUnparseableContext(t *testing.T) {
	// Never fail a keypress over a context we could not read.
	env := map[string]string{"HERDR_PLUGIN_CONTEXT_JSON": "not json at all"}
	if id, cwd := PaneTarget(func(k string) string { return env[k] }); id != "" || cwd != "" {
		t.Errorf("PaneTarget() = (%q, %q), want empty strings", id, cwd)
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

func TestNotifyArgsUsesAppIconForTheHostTerminal(t *testing.T) {
	// -appIcon, not -sender: -sender relies on a private bundle-id spoofing API
	// that macOS has closed off, and a notification posted that way never
	// arrives at all.
	got := NotifyArgs("npm run dev", "finished after 8s", "/Applications/Ghostty.app/Contents/Resources/Ghostty.icns")
	want := []string{
		"-title", "scratch",
		"-subtitle", "npm run dev",
		"-message", "finished after 8s",
		"-group", "herdr-scratch",
		"-appIcon", "/Applications/Ghostty.app/Contents/Resources/Ghostty.icns",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NotifyArgs() = %#v, want %#v", got, want)
	}
}

func TestNotifyArgsOmitsIconWhenItCannotBeResolved(t *testing.T) {
	// Passing -appIcon with an empty value makes terminal-notifier reject the
	// whole invocation, so an unresolved icon has to drop the flag entirely.
	got := NotifyArgs("build", "done", "")
	for _, arg := range got {
		if arg == "-appIcon" {
			t.Fatalf("NotifyArgs() included -appIcon with no icon: %#v", got)
		}
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
